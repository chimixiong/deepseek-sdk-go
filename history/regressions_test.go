package history_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go/chat"
	"github.com/chimixiong/deepseek-sdk-go/history"
	"github.com/chimixiong/deepseek-sdk-go/messages"
	"github.com/chimixiong/deepseek-sdk-go/responses"
)

func TestToChatOutputPassesValidate(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{{
		Role: history.RoleUser,
		Parts: []history.Part{
			history.Text("summarize"),
			history.FilePart(history.File{FileID: "file-1", Filename: "report.pdf"}),
		},
	}}}
	msgs, warnings := history.ToChat(transcript, history.Options{})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	req := &chat.Request{Model: "deepseek-flash", Messages: msgs}
	if err := req.Validate(); err != nil {
		t.Fatalf("ToChat output does not validate: %v", err)
	}
	for _, msg := range msgs {
		for _, part := range msg.Content.Parts {
			if part.File != nil && part.File.Filename != "" {
				t.Fatalf("file_id was paired with filename: %+v", part.File)
			}
		}
	}
}

func TestToMessagesOutputPassesValidate(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{
		{
			Role: history.RoleAssistant,
			Parts: []history.Part{
				history.Call("call_1", "read_document", json.RawMessage(`{}`)),
			},
		},
		{
			Role: history.RoleUser,
			Parts: []history.Part{
				history.Result("call_1", history.FilePart(history.File{FileID: "file-1"})),
			},
		},
	}}
	system, msgs, warnings := history.ToMessages(transcript, history.Options{})
	if system != nil {
		t.Fatalf("system = %+v, want nil", system)
	}
	foundDrop := false
	for _, warning := range warnings {
		if warning.Field == "tool_result.content" {
			foundDrop = true
		}
	}
	if !foundDrop {
		t.Fatalf("warnings = %+v, want a dropped document warning", warnings)
	}
	req := &messages.Request{Model: "deepseek-flash", Messages: msgs}
	if err := req.Validate(); err != nil {
		t.Fatalf("ToMessages output does not validate: %v", err)
	}
}

func TestSystemWarningsUseMinusOne(t *testing.T) {
	_, warnings := history.FromChat([]chat.Message{
		{Role: chat.RoleSystem, Content: chat.PartsContent(chat.TextPart("a"))},
		chat.UserMessage("hi"),
	})
	for _, warning := range warnings {
		if strings.HasPrefix(warning.Field, "system") && warning.Turn != -1 {
			t.Fatalf("system warning = %+v, want Turn -1", warning)
		}
	}
}

func TestToChatMergesAdjacentUserTurns(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{
		{Role: history.RoleUser, Parts: []history.Part{history.Text("first")}},
		{Role: history.RoleUser, Parts: []history.Part{history.Text("second")}},
	}}
	msgs, warnings := history.ToChat(transcript, history.Options{})
	if warnings != nil {
		t.Fatalf("warnings = %+v", warnings)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages = %+v, want one merged message", msgs)
	}
	var content strings.Builder
	if msgs[0].Content.Parts == nil {
		content.WriteString(msgs[0].Content.Text)
	} else {
		for _, part := range msgs[0].Content.Parts {
			content.WriteString(part.Text)
		}
	}
	if got := content.String(); got != "firstsecond" {
		t.Fatalf("content = %q", got)
	}
}

func TestFromMessagesResetsPendingBetweenAssistantTurns(t *testing.T) {
	msgs := []messages.Message{
		{
			Role: messages.RoleAssistant,
			Content: messages.Content{Blocks: []messages.Block{
				{Type: messages.BlockToolUse, ID: "old", Name: "f"},
			}},
		},
		{Role: messages.RoleUser, Content: messages.Content{Text: "next question"}},
		{Role: messages.RoleAssistant, Content: messages.Content{Text: "answer"}},
		{
			Role: messages.RoleUser,
			Content: messages.Content{Blocks: []messages.Block{
				{Type: messages.BlockToolResult, ToolUseID: "old"},
			}},
		},
	}
	transcript, warnings := history.FromMessages(msgs, nil)
	found := false
	for _, warning := range warnings {
		if strings.Contains(warning.Reason, `no matching tool_use`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %+v, want the stale result to be dropped", warnings)
	}
	for _, turn := range transcript.Turns {
		for _, part := range turn.Parts {
			if part.ToolResult != nil {
				t.Fatalf("stale tool result was paired: %+v", part.ToolResult)
			}
		}
	}
}

func TestFromResponseKeepsOutputSystemAndInstructions(t *testing.T) {
	instructions := "instructions"
	response := &responses.Response{
		Instructions: &instructions,
		Output: []responses.OutputItem{{
			Type: responses.ItemMessage,
			Role: responses.RoleSystem,
			Content: []responses.ContentPart{
				responses.OutputTextPart("from output"),
			},
		}},
	}
	transcript, warnings := history.FromResponse(response)
	if !strings.Contains(transcript.System, "from output") || !strings.Contains(transcript.System, "instructions") {
		t.Fatalf("system = %q", transcript.System)
	}
	for _, warning := range warnings {
		if warning.Turn != -1 {
			t.Fatalf("warning = %+v, want Turn -1", warning)
		}
	}
}
