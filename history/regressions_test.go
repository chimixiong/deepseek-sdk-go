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

func TestToChatEmptyToolResultValidates(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{
		{Role: history.RoleAssistant, Parts: []history.Part{
			history.Call("call_1", "read_document", json.RawMessage(`{}`)),
		}},
		{Role: history.RoleUser, Parts: []history.Part{history.Result("call_1")}},
	}}
	messages, warnings := history.ToChat(transcript, history.Options{})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if messages[1].Content.Text != "" || messages[1].Content.Parts != nil {
		t.Fatalf("tool content = %+v, want an empty string", messages[1].Content)
	}
	if err := (&chat.Request{Model: "deepseek-flash", Messages: messages}).Validate(); err != nil {
		t.Fatalf("ToChat output does not validate: %v", err)
	}
}

func TestToChatDropsEmptyFile(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{{
		Role:  history.RoleUser,
		Parts: []history.Part{history.Text("hi"), history.FilePart(history.File{})},
	}}}
	messages, warnings := history.ToChat(transcript, history.Options{})
	found := false
	for _, warning := range warnings {
		if warning.Field == "file" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %+v, want a file warning", warnings)
	}
	if len(messages[0].Content.Parts) != 0 {
		t.Fatalf("parts = %+v, want the empty file dropped", messages[0].Content.Parts)
	}
}

func TestToChatNormalizesInvalidArguments(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{
		{Role: history.RoleAssistant, Parts: []history.Part{history.Call("call_1", "f", nil)}},
		{Role: history.RoleUser, Parts: []history.Part{history.Result("call_1", history.Text("ok"))}},
	}}
	messages, warnings := history.ToChat(transcript, history.Options{})
	if got := messages[0].ToolCalls[0].Function.Arguments; got != "{}" {
		t.Fatalf("arguments = %q, want {}", got)
	}
	found := false
	for _, warning := range warnings {
		if warning.Field == "tool_call.arguments" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %+v, want an arguments warning", warnings)
	}
	if err := (&chat.Request{Model: "deepseek-flash", Messages: messages}).Validate(); err != nil {
		t.Fatalf("ToChat output does not validate: %v", err)
	}
}

func TestToMessagesDropsEmptyText(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{{
		Role:  history.RoleAssistant,
		Parts: []history.Part{history.Text("")},
	}}}
	_, messages, warnings := history.ToMessages(transcript, history.Options{})
	if len(messages) != 0 {
		t.Fatalf("messages = %+v, want the empty turn dropped", messages)
	}
	found := false
	for _, warning := range warnings {
		if warning.Field == "text" || warning.Field == "assistant" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %+v, want an empty text warning", warnings)
	}
}

func TestInlineImageMediaTypeIsDetected(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00")
	transcript := history.Transcript{Turns: []history.Turn{{
		Role:  history.RoleUser,
		Parts: []history.Part{history.ImagePart(history.Image{Data: png})},
	}}}
	_, messages, _ := history.ToMessages(transcript, history.Options{})
	source := messages[0].Content.Blocks[0].Source
	if source.MediaType != "image/png" {
		t.Fatalf("messages media type = %q, want image/png", source.MediaType)
	}
	chatMessages, _ := history.ToChat(transcript, history.Options{})
	if got := chatMessages[0].Content.Parts[0].ImageURL.URL; !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("chat data URL = %q, want image/png", got)
	}
}

func TestUnknownInlineImageMediaTypeIsDropped(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{{
		Role:  history.RoleUser,
		Parts: []history.Part{history.ImagePart(history.Image{Data: []byte("not an image")})},
	}}}
	_, messages, warnings := history.ToMessages(transcript, history.Options{})
	if len(messages) != 0 {
		t.Fatalf("messages = %+v, want the image dropped", messages)
	}
	found := false
	for _, warning := range warnings {
		if warning.Field == "image" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %+v, want an image warning", warnings)
	}
}
