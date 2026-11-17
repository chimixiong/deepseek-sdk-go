package history_test

import (
	"encoding/json"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go/history"
	"github.com/chimixiong/deepseek-sdk-go/responses"
)

func TestToResponses(t *testing.T) {
	transcript := history.Transcript{
		System: "be brief",
		Turns: []history.Turn{
			{Role: history.RoleUser, Parts: []history.Part{
				history.Text("weather?"),
				history.ImagePart(history.Image{URL: "https://example.com/a.png", Detail: "low"}),
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

	instructions, items, warnings := history.ToResponses(transcript, history.Options{})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if instructions != "be brief" {
		t.Errorf("instructions = %q", instructions)
	}
	if len(items) != 4 {
		t.Fatalf("items = %d: %+v", len(items), items)
	}
	if items[0].Type != responses.ItemMessage || items[0].Role != responses.RoleUser {
		t.Errorf("user item = %+v", items[0])
	}
	if items[0].Content == nil || len(items[0].Content.Parts) != 2 || items[0].Content.Parts[1].Type != responses.PartInputImage {
		t.Errorf("user content = %+v", items[0].Content)
	}
	if items[1].Type != responses.ItemMessage || items[1].Role != responses.RoleAssistant ||
		len(items[1].Content.Parts) != 1 || items[1].Content.Parts[0].Type != responses.PartOutputText {
		t.Errorf("assistant item = %+v", items[1])
	}
	if items[2].Type != responses.ItemFunctionCall || items[2].CallID != "call_1" || items[2].Name != "get_weather" {
		t.Errorf("function call = %+v", items[2])
	}
	if items[3].Type != responses.ItemFunctionCallOutput || items[3].CallID != "call_1" ||
		items[3].Output == nil || items[3].Output.Text != "sunny" {
		t.Errorf("function output = %+v", items[3])
	}
}

func TestFromResponse(t *testing.T) {
	response := &responses.Response{
		Instructions: new("sys"),
		Output: []responses.OutputItem{
			{Type: responses.ItemReasoning, Content: []responses.ContentPart{{Type: responses.PartReasoningText, Text: "think"}}},
			{Type: responses.ItemMessage, Role: responses.RoleAssistant, Content: []responses.ContentPart{{Type: responses.PartOutputText, Text: "hello"}}},
			{Type: responses.ItemFunctionCall, CallID: "call_1", Name: "get_weather", Arguments: `{"city":"Hangzhou"}`},
		},
	}
	transcript, warnings := history.FromResponse(response)
	if transcript.System != "sys" {
		t.Errorf("system = %q", transcript.System)
	}
	if len(warnings) != 1 || warnings[0].Field != "reasoning" {
		t.Errorf("warnings = %v", warnings)
	}
	if len(transcript.Turns) != 1 || transcript.Turns[0].Role != history.RoleAssistant {
		t.Fatalf("turns = %+v", transcript.Turns)
	}
	parts := transcript.Turns[0].Parts
	if len(parts) != 2 || parts[1].ToolCall == nil || parts[1].ToolCall.ID != "call_1" {
		t.Fatalf("parts = %+v", parts)
	}
}

func TestFromResponsesInput(t *testing.T) {
	content := responses.TextContent("weather?")
	output := responses.TextContent("sunny")
	items := []responses.InputItem{
		{Type: responses.ItemMessage, Role: responses.RoleUser, Content: &content},
		{Type: responses.ItemFunctionCall, CallID: "call_1", Name: "get_weather", Arguments: `{}`},
		{Type: responses.ItemFunctionCallOutput, CallID: "call_1", Output: &output},
	}
	transcript, warnings := history.FromResponsesInput(items)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(transcript.Turns) != 3 {
		t.Fatalf("turns = %+v", transcript.Turns)
	}
	result := transcript.Turns[2].Parts[0].ToolResult
	if result == nil || result.CallID != "call_1" || history.Texts(result.Parts) != "sunny" {
		t.Fatalf("result = %+v", result)
	}
}

func TestResponsesRoundTrip(t *testing.T) {
	original := history.Transcript{
		System: "sys",
		Turns: []history.Turn{
			{Role: history.RoleUser, Parts: []history.Part{history.Text("hi")}},
			{Role: history.RoleAssistant, Parts: []history.Part{
				history.Text("calling"),
				history.Call("call_1", "tool", json.RawMessage(`{}`)),
			}},
			{Role: history.RoleUser, Parts: []history.Part{history.Result("call_1", history.Text("ok"))}},
		},
	}
	instructions, items, warnings := history.ToResponses(original, history.Options{})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	inputItems := make([]responses.InputItem, 0, len(items))
	inputItems = append(inputItems, items...)
	back, backWarnings := history.FromResponsesInput(inputItems)
	if len(backWarnings) != 0 {
		t.Fatalf("back warnings = %v", backWarnings)
	}
	if instructions != "sys" {
		t.Errorf("instructions = %q", instructions)
	}
	if len(back.Turns) != 3 {
		t.Errorf("turns = %d: %+v", len(back.Turns), back.Turns)
	}
}

func TestToResponsesDropToolResults(t *testing.T) {
	transcript := history.Transcript{Turns: []history.Turn{
		{Role: history.RoleUser, Parts: []history.Part{history.Text("hi")}},
		{Role: history.RoleAssistant, Parts: []history.Part{history.Call("call_1", "tool", json.RawMessage(`{}`))}},
		{Role: history.RoleUser, Parts: []history.Part{
			history.Result("call_1", history.Text("ok")),
			history.Result("call_missing", history.Text("stray")),
		}},
	}}

	_, kept, warnings := history.ToResponses(transcript, history.Options{})
	if ids := functionOutputIDs(kept); len(ids) != 2 || ids[0] != "call_1" || ids[1] != "call_missing" {
		t.Fatalf("output ids = %v", ids)
	}
	if kept[2].Output.Text != "ok" {
		t.Errorf("paired output = %+v", kept[2])
	}
	if !hasWarning(warnings, "tool_result.call_id") {
		t.Errorf("warnings = %v", warnings)
	}

	_, dropped, warnings := history.ToResponses(transcript, history.Options{DropToolResults: true})
	if ids := functionOutputIDs(dropped); len(ids) != 1 || ids[0] != "call_1" {
		t.Fatalf("output ids = %v", ids)
	}
	if dropped[2].Output.Text != "ok" {
		t.Errorf("paired output = %+v", dropped[2])
	}
	if !hasWarning(warnings, "tool_result") {
		t.Errorf("warnings = %v", warnings)
	}
}

func functionOutputIDs(items []responses.InputItem) []string {
	var ids []string
	for _, item := range items {
		if item.Type == responses.ItemFunctionCallOutput {
			ids = append(ids, item.CallID)
		}
	}
	return ids
}
