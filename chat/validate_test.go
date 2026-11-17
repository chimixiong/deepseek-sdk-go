package chat_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go/chat"
)

func validRequest() *chat.Request {
	return &chat.Request{
		Model:    "deepseek-flash",
		Messages: []chat.Message{chat.UserMessage("hello")},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*chat.Request)
		want   string
	}{
		{"valid", func(*chat.Request) {}, ""},
		{"model", func(r *chat.Request) { r.Model = "" }, "model"},
		{"messages", func(r *chat.Request) { r.Messages = nil }, "messages"},
		{"role", func(r *chat.Request) { r.Messages[0].Role = "narrator" }, "role"},
		{"tool id", func(r *chat.Request) {
			r.Messages = append(r.Messages, chat.ToolMessage("", chat.TextContent("x")))
		}, "tool_call_id"},
		{"image url", func(r *chat.Request) {
			r.Messages[0] = chat.UserContentMessage(chat.ContentPart{Type: chat.PartImage, ImageURL: &chat.ImageURL{}})
		}, "image_url.url"},
		{"file exclusive", func(r *chat.Request) {
			r.Messages[0] = chat.UserContentMessage(chat.FilePart(chat.FileRef{FileID: "a", FileData: "b"}))
		}, "file_id or file_data"},
		{"top_logprobs", func(r *chat.Request) { r.TopLogprobs = new(2) }, "logprobs"},
		{"stream options", func(r *chat.Request) {
			r.StreamOptions = &chat.StreamOptions{IncludeUsage: new(true)}
		}, "CreateStream"},
		{"thinking required", func(r *chat.Request) {
			r.Thinking = &chat.Thinking{Type: chat.ThinkingEnabled}
			r.ToolChoice = chat.RequiredToolChoice()
		}, "thinking mode"},
		{"prefix on user", func(r *chat.Request) { r.Messages[0].Prefix = new(true) }, "prefix"},
		{"stop length", func(r *chat.Request) {
			r.Stop = chat.Stop{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q"}
		}, "stop"},
		{"stop empty", func(r *chat.Request) { r.Stop = chat.Stop{"END", ""} }, "empty"},
		{"user tool_call_id", func(r *chat.Request) { r.Messages[0].ToolCallID = "call_1" }, "tool_call_id"},
		{"user tool_calls", func(r *chat.Request) {
			r.Messages[0].ToolCalls = []chat.ToolCall{{ID: "call_1", Type: "function"}}
		}, "tool_calls"},
		{"latest_reminder format", func(r *chat.Request) {
			r.Messages = []chat.Message{chat.LatestReminderMessage("garbage"), chat.UserMessage("hi")}
		}, "date"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validRequest()
			tc.mutate(req)
			err := req.Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestMessageExtra(t *testing.T) {
	message := chat.UserMessage("hi")
	message.Extra = map[string]json.RawMessage{"custom": json.RawMessage(`{"a":1}`)}
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(fields["custom"]) != `{"a":1}` {
		t.Fatalf("extra = %s", fields["custom"])
	}
}

func TestToolChoiceValidation(t *testing.T) {
	req := validRequest()
	req.ToolChoice = &chat.ToolChoice{}
	if err := req.Validate(); err == nil {
		t.Fatal("expected an error for an empty tool choice")
	}
	req.ToolChoice = chat.FunctionToolChoice("get_weather")
	if err := req.Validate(); err != nil {
		t.Fatalf("named choice: %v", err)
	}
	req.Thinking = &chat.Thinking{Type: chat.ThinkingEnabled}
	if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "thinking mode") {
		t.Fatalf("err = %v", err)
	}
}

func TestLatestReminderValidation(t *testing.T) {
	const valid = "2026-11-16, 星期一, Singapore, Web, English"
	req := &chat.Request{
		Model:    "deepseek-flash",
		Messages: []chat.Message{chat.LatestReminderMessage(valid), chat.UserMessage("hi")},
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("valid reminder: %v", err)
	}

	req.Messages = []chat.Message{chat.UserMessage("hi"), chat.LatestReminderMessage(valid)}
	if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "immediately followed") {
		t.Fatalf("adjacency err = %v", err)
	}

	req.Messages = []chat.Message{chat.LatestReminderMessage("2026-11-16,星期一,Singapore,Web"), chat.UserMessage("hi")}
	if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "date") {
		t.Fatalf("field count err = %v", err)
	}

	req.Messages = []chat.Message{chat.LatestReminderMessage("2026-11-16,,Singapore,Web,English"), chat.UserMessage("hi")}
	if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "date") {
		t.Fatalf("empty field err = %v", err)
	}
}

func TestLatestReminderMustBeLastButOne(t *testing.T) {
	req := &chat.Request{
		Model: "deepseek-flash",
		Messages: []chat.Message{
			chat.UserMessage("hi"),
			chat.LatestReminderMessage("2026-11-16,星期一,Singapore,Web,English"),
		},
	}
	if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "immediately followed") {
		t.Fatalf("last reminder err = %v", err)
	}
}
