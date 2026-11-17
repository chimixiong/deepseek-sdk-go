package history_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go/chat"
	"github.com/chimixiong/deepseek-sdk-go/history"
)

func TestFromChat(t *testing.T) {
	messages := []chat.Message{
		chat.SystemMessage("be brief"),
		chat.UserContentMessage(
			chat.TextPart("what is this?"),
			chat.ImagePart("https://example.com/a.jpg", "low"),
		),
		chat.AssistantToolCalls("need weather", chat.ToolCall{
			ID: "call_1", Type: "function",
			Function: chat.FunctionCall{Name: "get_weather", Arguments: `{"city":"Hangzhou"}`},
		}),
		chat.ToolMessage("call_1", chat.TextContent("sunny")),
	}

	transcript, warnings := history.FromChat(messages)
	if transcript.System != "be brief" {
		t.Errorf("system = %q", transcript.System)
	}
	if len(transcript.Turns) != 3 {
		t.Fatalf("turns = %d: %+v", len(transcript.Turns), transcript.Turns)
	}
	if transcript.Turns[0].Role != history.RoleUser || len(transcript.Turns[0].Parts) != 2 {
		t.Errorf("user turn = %+v", transcript.Turns[0])
	}
	assistant := transcript.Turns[1]
	if assistant.Role != history.RoleAssistant || assistant.Parts[0].ToolCall == nil {
		t.Fatalf("assistant turn = %+v", assistant)
	}
	if string(assistant.Parts[0].ToolCall.Arguments) != `{"city":"Hangzhou"}` {
		t.Errorf("arguments = %s", assistant.Parts[0].ToolCall.Arguments)
	}
	result := transcript.Turns[2]
	if result.Parts[0].ToolResult == nil || result.Parts[0].ToolResult.CallID != "call_1" {
		t.Fatalf("tool result = %+v", result)
	}
	if !hasWarning(warnings, "reasoning_content") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestFromChatDropsUnpairedToolResult(t *testing.T) {
	transcript, warnings := history.FromChat([]chat.Message{
		chat.UserMessage("hi"),
		chat.ToolMessage("call_missing", chat.TextContent("x")),
	})
	if len(transcript.Turns) != 1 {
		t.Fatalf("turns = %+v", transcript.Turns)
	}
	if !hasWarning(warnings, "tool.tool_call_id") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestToChat(t *testing.T) {
	transcript := history.Transcript{
		System: "be brief",
		Turns: []history.Turn{
			{Role: history.RoleUser, Parts: []history.Part{
				history.Text("weather?"),
				history.ImagePart(history.Image{Data: []byte{1, 2}, MediaType: "image/png"}),
			}},
			{Role: history.RoleAssistant, Parts: []history.Part{
				history.Text("checking"),
				history.Call("call_1", "get_weather", json.RawMessage(`{"city":"Hangzhou"}`)),
			}},
			{Role: history.RoleUser, Parts: []history.Part{
				history.Result("call_1", history.Text("sunny")),
			}},
		},
	}

	messages, warnings := history.ToChat(transcript, history.Options{})
	if len(warnings) != 0 {
		t.Errorf("warnings = %v", warnings)
	}
	if len(messages) != 4 {
		t.Fatalf("messages = %d: %+v", len(messages), messages)
	}
	if messages[0].Role != chat.RoleSystem || messages[0].Content.Text != "be brief" {
		t.Errorf("system = %+v", messages[0])
	}
	if messages[1].Content.Parts == nil || messages[1].Content.Parts[1].ImageURL.URL == "" {
		t.Errorf("image content = %+v", messages[1])
	}
	if len(messages[2].ToolCalls) != 1 || messages[2].ToolCalls[0].Function.Arguments != `{"city":"Hangzhou"}` {
		t.Errorf("tool call = %+v", messages[2])
	}
	if messages[3].Role != chat.RoleTool || messages[3].ToolCallID != "call_1" || messages[3].Content.Text != "sunny" {
		t.Errorf("tool message = %+v", messages[3])
	}
}

func TestToChatDropImages(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{{
		Role: history.RoleUser,
		Parts: []history.Part{
			history.Text("look"),
			history.ImagePart(history.Image{URL: "https://example.com/a.png"}),
		},
	}}}
	messages, warnings := history.ToChat(transcript, history.Options{DropImages: true})
	if len(warnings) != 1 || !strings.Contains(warnings[0].String(), "image") {
		t.Fatalf("warnings = %v", warnings)
	}
	if messages[0].Content.Parts != nil || messages[0].Content.Text != "look" {
		t.Fatalf("content = %+v", messages[0].Content)
	}
}

func TestChatRoundTrip(t *testing.T) {
	original := history.Transcript{
		System: "sys",
		Turns: []history.Turn{
			{Role: history.RoleUser, Parts: []history.Part{history.Text("hi")}},
			{Role: history.RoleAssistant, Parts: []history.Part{
				history.Call("call_1", "tool", json.RawMessage(`{}`)),
			}},
			{Role: history.RoleUser, Parts: []history.Part{history.Result("call_1", history.Text("ok"))}},
			{Role: history.RoleAssistant, Parts: []history.Part{history.Text("done")}},
		},
	}
	messages, warnings := history.ToChat(original, history.Options{})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	back, backWarnings := history.FromChat(messages)
	if len(backWarnings) != 0 {
		t.Fatalf("back warnings = %v", backWarnings)
	}
	if back.System != "sys" {
		t.Errorf("system = %q", back.System)
	}
	if got := history.Texts(back.Turns[len(back.Turns)-1].Parts); got != "done" {
		t.Errorf("last turn = %q", got)
	}
	if len(back.Turns) != len(original.Turns) {
		t.Errorf("turns = %d, want %d", len(back.Turns), len(original.Turns))
	}
}

func hasWarning(warnings []history.Warning, field string) bool {
	for _, warning := range warnings {
		if warning.Field == field {
			return true
		}
	}
	return false
}

func TestToChatDropToolResults(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{
		{Role: history.RoleUser, Parts: []history.Part{history.Text("hi")}},
		{Role: history.RoleAssistant, Parts: []history.Part{history.Call("call_1", "tool", json.RawMessage(`{}`))}},
		{Role: history.RoleUser, Parts: []history.Part{
			history.Result("call_1", history.Text("ok")),
			history.Result("call_missing", history.Text("stray")),
		}},
	}}

	kept, warnings := history.ToChat(transcript, history.Options{})
	if ids := chatToolIDs(kept); len(ids) != 2 || ids[0] != "call_1" || ids[1] != "call_missing" {
		t.Fatalf("tool ids = %v", ids)
	}
	if kept[2].Content.Text != "ok" {
		t.Errorf("paired result content = %+v", kept[2])
	}
	if !hasWarning(warnings, "tool_result.call_id") {
		t.Errorf("warnings = %v", warnings)
	}

	dropped, warnings := history.ToChat(transcript, history.Options{DropToolResults: true})
	if ids := chatToolIDs(dropped); len(ids) != 1 || ids[0] != "call_1" {
		t.Fatalf("tool ids = %v", ids)
	}
	if dropped[2].Content.Text != "ok" {
		t.Errorf("paired result content = %+v", dropped[2])
	}
	if !hasWarning(warnings, "tool_result") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestFromChatDataURLs(t *testing.T) {
	cases := []struct {
		name      string
		fileData  string
		wantData  string
		wantMedia string
		wantLoss  bool
	}{
		{name: "base64 with parameters", fileData: "data:text/plain;charset=utf-8;base64,aGVsbG8=", wantData: "hello", wantMedia: "text/plain"},
		{name: "plain percent-encoded", fileData: "data:text/plain,hi", wantData: "hi", wantMedia: "text/plain"},
		{name: "missing comma", fileData: "data:text/plain;base64", wantLoss: true},
		{name: "invalid base64", fileData: "data:text/plain;base64,!!!", wantLoss: true},
		{name: "empty reference", fileData: "", wantLoss: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transcript, warnings := history.FromChat([]chat.Message{
				chat.UserContentMessage(chat.FilePart(chat.FileRef{FileData: tc.fileData, Filename: "a.txt"})),
			})
			if tc.wantLoss {
				if len(transcript.Turns) != 0 {
					t.Fatalf("turns = %+v", transcript.Turns)
				}
				if !hasWarning(warnings, "file") {
					t.Fatalf("warnings = %v", warnings)
				}
				if strings.Contains(warnings[0].Reason, "empty file reference") {
					t.Errorf("warning names the wrong cause: %q", warnings[0].Reason)
				}
				return
			}
			if len(warnings) != 0 {
				t.Fatalf("warnings = %v", warnings)
			}
			file := transcript.Turns[0].Parts[0].File
			if file == nil || string(file.Data) != tc.wantData || file.MediaType != tc.wantMedia {
				t.Fatalf("file = %+v", file)
			}
		})
	}
}

func chatToolIDs(messages []chat.Message) []string {
	var ids []string
	for _, message := range messages {
		if message.Role == chat.RoleTool {
			ids = append(ids, message.ToolCallID)
		}
	}
	return ids
}

func TestEmptyImageReferenceDropped(t *testing.T) {
	transcript, warnings := history.FromChat([]chat.Message{
		chat.UserContentMessage(chat.TextPart("look"), chat.ImagePart("", "")),
	})
	if !hasWarning(warnings, "image_url.url") {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(transcript.Turns) != 1 || len(transcript.Turns[0].Parts) != 1 {
		t.Fatalf("turns = %+v", transcript.Turns)
	}

	messages, warnings := history.ToChat(history.Transcript{Turns: []history.Turn{{
		Role:  history.RoleUser,
		Parts: []history.Part{history.Text("look"), history.ImagePart(history.Image{})},
	}}}, history.Options{})
	if !hasWarning(warnings, "image") {
		t.Fatalf("warnings = %v", warnings)
	}
	for _, message := range messages {
		for _, part := range message.Content.Parts {
			if part.Type == chat.PartImage {
				t.Fatalf("fabricated image part: %+v", part)
			}
		}
	}
}
