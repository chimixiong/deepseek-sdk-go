package history_test

import (
	"encoding/json"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go/chat"
	"github.com/chimixiong/deepseek-sdk-go/history"
	"github.com/chimixiong/deepseek-sdk-go/messages"
)

func TestToMessages(t *testing.T) {
	transcript := history.Transcript{
		System: "be brief",
		Turns: []history.Turn{
			{Role: history.RoleUser, Parts: []history.Part{
				history.Text("describe"),
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

	system, msgs, warnings := history.ToMessages(transcript, history.Options{})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if system == nil || system.Text != "be brief" {
		t.Fatalf("system = %+v", system)
	}
	if len(msgs) != 3 {
		t.Fatalf("messages = %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Content.Blocks == nil || len(msgs[0].Content.Blocks) != 2 || msgs[0].Content.Blocks[1].Type != messages.BlockImage {
		t.Fatalf("user content = %+v", msgs[0].Content)
	}
	if msgs[0].Content.Blocks[1].Source.Type != messages.SourceBase64 || msgs[0].Content.Blocks[1].Source.MediaType != "image/png" {
		t.Fatalf("image source = %+v", msgs[0].Content.Blocks[1].Source)
	}
	assistant := msgs[1].Content.Blocks
	if len(assistant) != 2 || assistant[1].Type != messages.BlockToolUse || assistant[1].ID != "call_1" {
		t.Fatalf("assistant blocks = %+v", assistant)
	}
	if string(assistant[1].Input) != `{"city":"Hangzhou"}` {
		t.Fatalf("input = %s", assistant[1].Input)
	}
	result := msgs[2].Content.Blocks
	if len(result) != 1 || result[0].Type != messages.BlockToolResult || result[0].ToolUseID != "call_1" {
		t.Fatalf("result blocks = %+v", result)
	}
	if result[0].Content == nil || result[0].Content.Text != "sunny" {
		t.Fatalf("result content = %+v", result[0].Content)
	}
}

func TestToMessagesToolResultFirst(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{
		{
			Role:  history.RoleAssistant,
			Parts: []history.Part{history.Call("call_1", "tool", json.RawMessage(`{}`))},
		},
		{
			Role: history.RoleUser,
			Parts: []history.Part{
				history.Result("call_1", history.Text("ok")),
				history.Text("and now?"),
			},
		},
	}}
	_, msgs, warnings := history.ToMessages(transcript, history.Options{})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	blocks := msgs[1].Content.Blocks
	if len(blocks) != 2 || blocks[0].Type != messages.BlockToolResult || blocks[1].Type != messages.BlockText {
		t.Fatalf("blocks = %+v", blocks)
	}
}

func TestFromMessage(t *testing.T) {
	answer := "Here it is"
	response := &messages.Response{
		Content: []messages.Block{
			{Type: messages.BlockThinking, Thinking: "think", Signature: &answer},
			{Type: messages.BlockText, Text: "hello"},
			{Type: messages.BlockToolUse, ID: "call_1", Name: "get_weather", Input: json.RawMessage(`{"city":"Hangzhou"}`)},
		},
	}
	transcript, warnings := history.FromMessage(response)
	if len(warnings) != 1 || warnings[0].Field != "thinking" {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(transcript.Turns) != 1 {
		t.Fatalf("turns = %+v", transcript.Turns)
	}
	parts := transcript.Turns[0].Parts
	if len(parts) != 2 || parts[1].ToolCall == nil || parts[1].ToolCall.ID != "call_1" {
		t.Fatalf("parts = %+v", parts)
	}
}

func TestFromMessages(t *testing.T) {
	text := "sys"
	system := &messages.Content{Blocks: []messages.Block{{Type: messages.BlockText, Text: text}}}
	msgs := []messages.Message{
		{Role: messages.RoleUser, Content: messages.Content{Text: "hi"}},
		{Role: messages.RoleAssistant, Content: messages.Content{Blocks: []messages.Block{
			{Type: messages.BlockToolUse, ID: "call_1", Name: "tool", Input: json.RawMessage(`{}`)},
		}}},
		{Role: messages.RoleUser, Content: messages.Content{Blocks: []messages.Block{
			{Type: messages.BlockToolResult, ToolUseID: "call_1", Content: &messages.Content{Text: "ok"}},
		}}},
		{Role: messages.RoleUser, Content: messages.Content{Blocks: []messages.Block{
			{Type: messages.BlockToolResult, ToolUseID: "call_missing", Content: &messages.Content{Text: "x"}},
		}}},
	}
	transcript, warnings := history.FromMessages(msgs, system)
	if transcript.System != "sys" {
		t.Errorf("system = %q", transcript.System)
	}
	if len(transcript.Turns) != 3 {
		t.Fatalf("turns = %+v", transcript.Turns)
	}
	result := transcript.Turns[2].Parts[0].ToolResult
	if result == nil || result.CallID != "call_1" || history.Texts(result.Parts) != "ok" {
		t.Fatalf("result = %+v", result)
	}
	if !hasWarning(warnings, "tool_result.tool_use_id") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestMessagesRoundTrip(t *testing.T) {
	original := history.Transcript{
		System: "sys",
		Turns: []history.Turn{
			{Role: history.RoleUser, Parts: []history.Part{history.Text("hi")}},
			{Role: history.RoleAssistant, Parts: []history.Part{
				history.Text("calling"),
				history.Call("call_1", "tool", json.RawMessage(`{}`)),
			}},
			{Role: history.RoleUser, Parts: []history.Part{history.Result("call_1", history.Text("ok"))}},
			{Role: history.RoleAssistant, Parts: []history.Part{history.Text("done")}},
		},
	}
	system, msgs, warnings := history.ToMessages(original, history.Options{})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	back, backWarnings := history.FromMessages(msgs, system)
	if len(backWarnings) != 0 {
		t.Fatalf("back warnings = %v", backWarnings)
	}
	if back.System != "sys" {
		t.Errorf("system = %q", back.System)
	}
	if len(back.Turns) != len(original.Turns) {
		t.Fatalf("turns = %d, want %d: %+v", len(back.Turns), len(original.Turns), back.Turns)
	}
	if history.Texts(back.Turns[3].Parts) != "done" {
		t.Errorf("last turn = %+v", back.Turns[3])
	}
}

func TestToMessagesDropToolResults(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{
		{Role: history.RoleUser, Parts: []history.Part{history.Text("hi")}},
		{Role: history.RoleAssistant, Parts: []history.Part{history.Call("call_1", "tool", json.RawMessage(`{}`))}},
		{Role: history.RoleUser, Parts: []history.Part{
			history.Result("call_1", history.Text("ok")),
			history.Result("call_missing", history.Text("stray")),
		}},
	}}

	_, kept, warnings := history.ToMessages(transcript, history.Options{})
	if ids := messagesToolResultIDs(kept); len(ids) != 2 || ids[0] != "call_1" || ids[1] != "call_missing" {
		t.Fatalf("tool_use_ids = %v", ids)
	}
	if kept[2].Content.Blocks[0].Content.Text != "ok" {
		t.Errorf("paired result content = %+v", kept[2].Content.Blocks[0])
	}
	if !hasWarning(warnings, "tool_result.tool_use_id") {
		t.Errorf("warnings = %v", warnings)
	}

	_, dropped, warnings := history.ToMessages(transcript, history.Options{DropToolResults: true})
	if ids := messagesToolResultIDs(dropped); len(ids) != 1 || ids[0] != "call_1" {
		t.Fatalf("tool_use_ids = %v", ids)
	}
	if dropped[2].Content.Blocks[0].Content.Text != "ok" {
		t.Errorf("paired result content = %+v", dropped[2].Content.Blocks[0])
	}
	if !hasWarning(warnings, "tool_result") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestToMessagesDropsToolCallInUserTurn(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{
		{Role: history.RoleAssistant, Parts: []history.Part{history.Call("call_1", "tool", json.RawMessage(`{}`))}},
		{Role: history.RoleUser, Parts: []history.Part{
			history.Text("hi"),
			history.Call("c1", "tool", json.RawMessage(`{}`)),
			history.Result("call_1", history.Text("ok"), history.Call("c2", "tool", json.RawMessage(`{}`))),
		}},
	}}

	system, msgs, warnings := history.ToMessages(transcript, history.Options{})
	if system != nil {
		t.Errorf("system = %+v", system)
	}
	if !hasWarning(warnings, "tool_call") || !hasWarning(warnings, "tool_use") {
		t.Errorf("warnings = %v", warnings)
	}
	request := messages.Request{Model: "claude-sonnet-4-5", Messages: msgs}
	if err := request.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for _, message := range msgs {
		for _, block := range message.Content.Blocks {
			if block.Type == messages.BlockToolUse && message.Role != messages.RoleAssistant {
				t.Fatalf("tool_use in %s message: %+v", message.Role, block)
			}
			if block.Content == nil {
				continue
			}
			for _, inner := range block.Content.Blocks {
				if inner.Type == messages.BlockToolUse {
					t.Fatalf("nested tool_use in tool_result: %+v", inner)
				}
			}
		}
	}
}

func TestMessagesImageBase64Loss(t *testing.T) {
	for _, tc := range []struct{ name, data string }{
		{name: "invalid base64", data: "!!!not-base64!!!"},
		{name: "empty data", data: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := messages.Source{Type: messages.SourceBase64, MediaType: "image/png", Data: tc.data}
			transcript, warnings := history.FromMessages([]messages.Message{{
				Role:    messages.RoleUser,
				Content: messages.Content{Blocks: []messages.Block{{Type: messages.BlockImage, Source: &source}}},
			}}, nil)
			if !hasWarning(warnings, "image.source") {
				t.Fatalf("warnings = %v", warnings)
			}
			chatMessages, _ := history.ToChat(transcript, history.Options{})
			for _, message := range chatMessages {
				for _, part := range message.Content.Parts {
					if part.Type == chat.PartImage {
						t.Fatalf("fabricated image part: %+v", part)
					}
				}
			}
		})
	}
}

func TestMessagesImageBase64RoundTrip(t *testing.T) {
	source := messages.Source{Type: messages.SourceBase64, MediaType: "image/png", Data: "AQI="}
	transcript, warnings := history.FromMessages([]messages.Message{{
		Role:    messages.RoleUser,
		Content: messages.Content{Blocks: []messages.Block{{Type: messages.BlockImage, Source: &source}}},
	}}, nil)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	chatMessages, chatWarnings := history.ToChat(transcript, history.Options{})
	if len(chatWarnings) != 0 {
		t.Fatalf("chat warnings = %v", chatWarnings)
	}
	request := chat.Request{Model: "deepseek-chat", Messages: chatMessages}
	if err := request.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	url := chatMessages[0].Content.Parts[0].ImageURL.URL
	if url != "data:image/png;base64,AQI=" {
		t.Fatalf("image url = %q", url)
	}

	system, msgs, warnings := history.ToMessages(transcript, history.Options{})
	if system != nil || len(warnings) != 0 {
		t.Fatalf("system = %+v warnings = %v", system, warnings)
	}
	block := msgs[0].Content.Blocks[0]
	if block.Type != messages.BlockImage || block.Source == nil || block.Source.Data != "AQI=" {
		t.Fatalf("image block = %+v", block)
	}
}

func messagesToolResultIDs(msgs []messages.Message) []string {
	var ids []string
	for _, message := range msgs {
		for _, block := range message.Content.Blocks {
			if block.Type == messages.BlockToolResult {
				ids = append(ids, block.ToolUseID)
			}
		}
	}
	return ids
}
