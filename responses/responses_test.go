package responses_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go"
	"github.com/chimixiong/deepseek-sdk-go/responses"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *responses.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return responses.New(deepseek.New(deepseek.Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		MaxRetries: -1,
	}))
}

func rawObject(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode object %s: %v", raw, err)
	}
	return object
}

func rawArray(t *testing.T, raw json.RawMessage) []json.RawMessage {
	t.Helper()
	var array []json.RawMessage
	if err := json.Unmarshal(raw, &array); err != nil {
		t.Fatalf("decode array %s: %v", raw, err)
	}
	return array
}

func rawString(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode string %s: %v", raw, err)
	}
	return value
}

// responseFixture covers every documented output item kind plus usage, title
// and an unmodelled top-level field.
const responseFixture = `{
  "id": "resp_1",
  "object": "response",
  "created_at": 1793084842,
  "status": "completed",
  "background": false,
  "completed_at": 1793084843,
  "content_filters": null,
  "error": null,
  "frequency_penalty": 0.0,
  "incomplete_details": null,
  "instructions": "Be concise.",
  "max_output_tokens": 1024,
  "max_tool_calls": null,
  "model": "deepseek-v4-pro",
  "moderation": null,
  "output": [
    {
      "type": "reasoning",
      "id": "rs_1",
      "status": "completed",
      "content": [{"type": "reasoning_text", "text": "Need current weather."}]
    },
    {
      "type": "message",
      "id": "msg_1",
      "status": "completed",
      "content": [
        {
          "type": "output_text",
          "annotations": [{"type": "url_citation", "url": "https://example.com"}],
          "logprobs": [],
          "text": "It is sunny in Paris."
        }
      ],
      "phase": "final_answer",
      "role": "assistant"
    },
    {
      "type": "function_call",
      "id": "fc_1",
      "status": "completed",
      "call_id": "call_1",
      "name": "get_weather",
      "arguments": "{\"city\":\"Paris\"}"
    },
    {
      "type": "custom_tool_call",
      "id": "ctc_1",
      "status": "completed",
      "call_id": "call_2",
      "name": "apply_patch",
      "input": "*** Begin Patch\n*** End Patch"
    },
    {
      "type": "web_search_call",
      "id": "ws_1",
      "status": "completed",
      "kind": "search",
      "action": {
        "type": "search",
        "queries": ["paris weather"],
        "sources": [
          {
            "type": "url",
            "url": "https://example.com/weather",
            "title": "Weather",
            "snippet": "Sunny",
            "site_name": "Example",
            "site_icon": "https://example.com/favicon.ico",
            "published_at": "2026-11-16T08:00:00+00:00"
          }
        ]
      },
      "page": {"url": "https://example.com/weather", "title": "Weather", "published_at": "2026-11-16"},
      "signature": "c2lnbmF0dXJl",
      "error": null
    },
    {
      "type": "web_search_call",
      "id": "ws_2",
      "status": "failed",
      "kind": "read_link",
      "action": {"type": "open_page", "url": "https://blocked.example"},
      "error": {"code": "url_not_allowed"}
    },
    {
      "type": "tool_search_call",
      "id": "tsc_1",
      "status": "completed",
      "call_id": "call_3",
      "x_extra": {"a": 1}
    }
  ],
  "parallel_tool_calls": true,
  "presence_penalty": 0.0,
  "previous_response_id": null,
  "prompt_cache_key": null,
  "prompt_cache_retention": null,
  "reasoning": {"effort": "high", "summary": null},
  "safety_identifier": null,
  "service_tier": "default",
  "store": false,
  "temperature": 0.2,
  "text": {"format": {"type": "json_schema", "name": "weather"}, "verbosity": null},
  "tool_choice": {"type": "function", "name": "get_weather"},
  "tools": [
    {"type": "function", "name": "get_weather", "description": "Get weather", "parameters": {"type": "object"}},
    {"type": "web_search"}
  ],
  "top_logprobs": 5,
  "top_p": 0.9,
  "truncation": "disabled",
  "usage": {
    "input_tokens": 3475,
    "input_tokens_details": {"cached_tokens": 512},
    "output_tokens": 54,
    "output_tokens_details": {"reasoning_tokens": 12, "safety_tokens": 3},
    "total_tokens": 3529,
    "server_tool_use": {"tool_search": 1, "tool_fetch": 2, "tool_image_search": 1}
  },
  "user": "user-1",
  "metadata": {},
  "title": "Weather in Paris",
  "x_trace": "trace-1"
}`

func TestCreateDocShapedRequest(t *testing.T) {
	var body map[string]json.RawMessage
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/responses" {
			t.Errorf("path = %s, want /responses", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content-type = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("accept = %q", got)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode body: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, responseFixture)
	})

	req := &responses.Request{
		Model: "deepseek-v4-pro",
		Input: responses.ItemsInput(
			responses.InputItem{
				Type: responses.ItemMessage,
				Role: responses.RoleUser,
				Content: new(responses.PartsContent(
					responses.InputTextPart("What is in this image?"),
					responses.InputImagePart("https://example.com/cat.png", responses.DetailLow),
					responses.InputFilePart("file-api-doc"),
				)),
			},
			responses.InputItem{
				Type:    responses.ItemMessage,
				Role:    responses.RoleSystem,
				Content: new(responses.TextContent("Be precise.")),
			},
			responses.InputItem{
				Type:    responses.ItemMessage,
				Role:    responses.RoleDeveloper,
				Content: new(responses.TextContent("Use metric units.")),
			},
			responses.InputItem{
				Type:    responses.ItemMessage,
				Role:    responses.RoleAssistant,
				Content: new(responses.TextContent("Sure.")),
			},
			responses.InputItem{
				Type:      responses.ItemFunctionCall,
				CallID:    "call_1",
				Name:      "get_weather",
				Arguments: `{"city":"Paris"}`,
			},
			responses.InputItem{
				Type:   responses.ItemFunctionCallOutput,
				CallID: "call_1",
				Output: new(responses.PartsContent(
					responses.InputTextPart("sunny"),
					responses.InputImageFilePart("file-api-image", responses.DetailAuto),
				)),
			},
			responses.InputItem{
				Type:   responses.ItemCustomToolCall,
				CallID: "call_2",
				Name:   "apply_patch",
				Input:  new("*** Begin Patch\n*** End Patch"),
			},
			responses.InputItem{
				Type:   responses.ItemCustomToolCallOutput,
				CallID: "call_2",
				Output: new(responses.TextContent("applied")),
			},
			responses.InputItem{
				Type:    responses.ItemReasoning,
				Content: new(responses.PartsContent(responses.ReasoningTextPart("Check the units."))),
			},
		),
		Instructions:    new("Be concise."),
		Reasoning:       &responses.Reasoning{Effort: responses.EffortHigh},
		MaxOutputTokens: new(1024),
		Stream:          new(true),
		Temperature:     new(0.2),
		TopP:            new(0.9),
		Text: &responses.Text{
			Format: &responses.TextFormat{
				Type:   responses.FormatJSONSchema,
				Name:   "weather",
				Schema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
			},
		},
		Tools: []responses.Tool{
			responses.FunctionTool("get_weather", "Get weather", map[string]any{"type": "object"}),
			responses.CustomTool("apply_patch"),
			responses.WebSearchTool(),
		},
		ToolChoice:  responses.FunctionToolChoice("get_weather"),
		TopLogprobs: new(5),
		User:        new("user-1"),
		SafetyCheck: &responses.SafetyCheck{Level: responses.SafetyHigh},
		Title:       &responses.Title{Enabled: true},
		Extra:       map[string]json.RawMessage{"prompt_cache_key": json.RawMessage(`"cache-1"`)},
	}

	resp, err := client.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got := rawString(t, body["model"]); got != "deepseek-v4-pro" {
		t.Errorf("model = %q", got)
	}
	if got := string(body["stream"]); got != "false" {
		t.Errorf("stream = %q, want false", got)
	}
	if got := rawString(t, body["instructions"]); got != "Be concise." {
		t.Errorf("instructions = %q", got)
	}
	if got := rawString(t, body["prompt_cache_key"]); got != "cache-1" {
		t.Errorf("extra prompt_cache_key = %q", got)
	}

	input := rawArray(t, body["input"])
	if len(input) != 9 {
		t.Fatalf("input length = %d, want 9", len(input))
	}
	first := rawObject(t, input[0])
	if got := rawString(t, first["type"]); got != "message" {
		t.Errorf("input[0].type = %q", got)
	}
	if got := rawString(t, first["role"]); got != "user" {
		t.Errorf("input[0].role = %q", got)
	}
	parts := rawArray(t, first["content"])
	if len(parts) != 3 {
		t.Fatalf("input[0].content length = %d, want 3", len(parts))
	}
	if got := rawString(t, rawObject(t, parts[0])["type"]); got != "input_text" {
		t.Errorf("part 0 type = %q", got)
	}
	image := rawObject(t, parts[1])
	if got := rawString(t, image["type"]); got != "input_image" {
		t.Errorf("part 1 type = %q", got)
	}
	if got := rawString(t, image["image_url"]); got != "https://example.com/cat.png" {
		t.Errorf("image url = %q", got)
	}
	if got := rawString(t, image["detail"]); got != "low" {
		t.Errorf("image detail = %q", got)
	}
	file := rawObject(t, parts[2])
	if got := rawString(t, file["file_id"]); got != "file-api-doc" {
		t.Errorf("file id = %q", got)
	}

	if got := rawString(t, rawObject(t, input[1])["role"]); got != "system" {
		t.Errorf("input[1].role = %q", got)
	}
	if got := rawString(t, rawObject(t, input[2])["role"]); got != "developer" {
		t.Errorf("input[2].role = %q", got)
	}
	if got := rawString(t, rawObject(t, input[3])["role"]); got != "assistant" {
		t.Errorf("input[3].role = %q", got)
	}

	call := rawObject(t, input[4])
	if got := rawString(t, call["type"]); got != "function_call" {
		t.Errorf("function call type = %q", got)
	}
	if got := rawString(t, call["call_id"]); got != "call_1" {
		t.Errorf("function call id = %q", got)
	}
	if got := rawString(t, call["arguments"]); got != `{"city":"Paris"}` {
		t.Errorf("function call arguments = %q", got)
	}

	output := rawArray(t, rawObject(t, input[5])["output"])
	outputImage := rawObject(t, output[1])
	if got := rawString(t, outputImage["file_id"]); got != "file-api-image" {
		t.Errorf("output image file id = %q", got)
	}
	if _, ok := outputImage["image_url"]; ok {
		t.Error("output image should not have image_url")
	}

	custom := rawObject(t, input[6])
	if got := rawString(t, custom["input"]); got != "*** Begin Patch\n*** End Patch" {
		t.Errorf("custom tool input = %q", got)
	}
	reasoning := rawObject(t, input[8])
	reasoningParts := rawArray(t, reasoning["content"])
	if got := rawString(t, rawObject(t, reasoningParts[0])["type"]); got != "reasoning_text" {
		t.Errorf("reasoning part type = %q", got)
	}

	tools := rawArray(t, body["tools"])
	if len(tools) != 3 {
		t.Fatalf("tools length = %d, want 3", len(tools))
	}
	functionTool := rawObject(t, tools[0])
	if got := rawString(t, functionTool["name"]); got != "get_weather" {
		t.Errorf("tool name = %q", got)
	}
	if _, nested := functionTool["function"]; nested {
		t.Error("Responses function tool must not nest fields under function")
	}
	if got := rawString(t, rawObject(t, functionTool["parameters"])["type"]); got != "object" {
		t.Errorf("tool parameters type = %q", got)
	}
	if got := rawString(t, rawObject(t, tools[1])["name"]); got != "apply_patch" {
		t.Errorf("custom tool name = %q", got)
	}
	if got := rawString(t, rawObject(t, tools[2])["type"]); got != "web_search" {
		t.Errorf("web search tool type = %q", got)
	}

	choice := rawObject(t, body["tool_choice"])
	if got := rawString(t, choice["type"]); got != "function" {
		t.Errorf("tool_choice.type = %q", got)
	}
	if got := rawString(t, choice["name"]); got != "get_weather" {
		t.Errorf("tool_choice.name = %q", got)
	}

	text := rawObject(t, body["text"])
	format := rawObject(t, text["format"])
	if got := rawString(t, format["type"]); got != "json_schema" {
		t.Errorf("text.format.type = %q", got)
	}
	if got := rawString(t, format["name"]); got != "weather" {
		t.Errorf("text.format.name = %q", got)
	}
	if got := rawString(t, rawObject(t, format["schema"])["type"]); got != "object" {
		t.Errorf("text.format.schema.type = %q", got)
	}

	if got := rawString(t, rawObject(t, body["reasoning"])["effort"]); got != "high" {
		t.Errorf("reasoning.effort = %q", got)
	}
	if got := rawString(t, rawObject(t, body["safety_check"])["level"]); got != "high" {
		t.Errorf("safety_check.level = %q", got)
	}
	if got := string(rawObject(t, body["title"])["enabled"]); got != "true" {
		t.Errorf("title.enabled = %q", got)
	}
	if got := rawString(t, body["user"]); got != "user-1" {
		t.Errorf("user = %q", got)
	}

	// Response decoding.
	if resp.ID != "resp_1" || resp.Object != "response" || resp.Status != responses.StatusCompleted {
		t.Errorf("response header = %+v", resp)
	}
	if resp.CreatedAt != 1793084842 || resp.CompletedAt == nil || *resp.CompletedAt != 1793084843 {
		t.Errorf("timestamps = %d, %v", resp.CreatedAt, resp.CompletedAt)
	}
	if resp.Title != "Weather in Paris" {
		t.Errorf("title = %q", resp.Title)
	}
	if got := resp.OutputText(); got != "It is sunny in Paris." {
		t.Errorf("output text = %q", got)
	}
	if got := rawString(t, resp.Extra["x_trace"]); got != "trace-1" {
		t.Errorf("extra x_trace = %q", got)
	}
	if len(resp.Output) != 7 {
		t.Fatalf("output length = %d, want 7", len(resp.Output))
	}

	reasoningItem := resp.Output[0]
	if reasoningItem.Type != responses.ItemReasoning || len(reasoningItem.Content) != 1 {
		t.Fatalf("reasoning item = %+v", reasoningItem)
	}
	if reasoningItem.Content[0].Type != responses.PartReasoningText || reasoningItem.Content[0].Text != "Need current weather." {
		t.Errorf("reasoning part = %+v", reasoningItem.Content[0])
	}

	message := resp.Output[1]
	if message.Type != responses.ItemMessage || message.Role != responses.RoleAssistant || message.Phase != "final_answer" {
		t.Errorf("message item = %+v", message)
	}
	if len(message.Content) != 1 || message.Content[0].Type != responses.PartOutputText {
		t.Fatalf("message content = %+v", message.Content)
	}
	if len(message.Content[0].Annotations) != 1 || !strings.Contains(string(message.Content[0].Annotations[0]), "url_citation") {
		t.Errorf("annotations = %s", message.Content[0].Annotations)
	}
	if message.Content[0].Logprobs == nil || len(message.Content[0].Logprobs) != 0 {
		t.Errorf("logprobs = %#v", message.Content[0].Logprobs)
	}

	functionCall := resp.Output[2]
	if functionCall.Type != responses.ItemFunctionCall || functionCall.CallID != "call_1" ||
		functionCall.Name != "get_weather" || functionCall.Arguments != `{"city":"Paris"}` {
		t.Errorf("function call item = %+v", functionCall)
	}

	customCall := resp.Output[3]
	if customCall.Type != responses.ItemCustomToolCall || customCall.Input == nil ||
		*customCall.Input != "*** Begin Patch\n*** End Patch" {
		t.Errorf("custom tool call item = %+v", customCall)
	}

	search := resp.Output[4]
	if search.Type != responses.ItemWebSearchCall || search.Kind != responses.KindSearch {
		t.Fatalf("web search item = %+v", search)
	}
	if search.Action == nil || search.Action.Type != responses.ActionSearch ||
		len(search.Action.Queries) != 1 || search.Action.Queries[0] != "paris weather" {
		t.Errorf("web search action = %+v", search.Action)
	}
	if len(search.Action.Sources) != 1 {
		t.Fatalf("sources = %+v", search.Action.Sources)
	}
	source := search.Action.Sources[0]
	if source.Type != "url" || source.SiteName != "Example" || source.PublishedAt != "2026-11-16T08:00:00+00:00" {
		t.Errorf("source = %+v", source)
	}
	if search.Page == nil || search.Page.URL != "https://example.com/weather" {
		t.Errorf("page = %+v", search.Page)
	}
	if search.Signature != "c2lnbmF0dXJl" {
		t.Errorf("signature = %q", search.Signature)
	}

	failedSearch := resp.Output[5]
	if failedSearch.Status != responses.ItemFailed || failedSearch.Error == nil ||
		failedSearch.Error.Code != responses.WebErrorURLNotAllowed {
		t.Errorf("failed web search = %+v", failedSearch)
	}

	toolSearch := resp.Output[6]
	if toolSearch.Type != responses.ItemToolSearchCall {
		t.Errorf("tool search type = %q", toolSearch.Type)
	}
	if !strings.Contains(string(toolSearch.Extra["x_extra"]), `"a": 1`) {
		t.Errorf("tool search extra = %s", toolSearch.Extra["x_extra"])
	}

	if resp.Usage == nil {
		t.Fatal("usage is nil")
	}
	usage := resp.Usage
	if usage.InputTokens != 3475 || usage.OutputTokens != 54 || usage.TotalTokens != 3529 {
		t.Errorf("usage = %+v", usage)
	}
	if usage.InputTokensDetails == nil || usage.InputTokensDetails.CachedTokens != 512 {
		t.Errorf("input details = %+v", usage.InputTokensDetails)
	}
	if usage.OutputTokensDetails == nil || usage.OutputTokensDetails.ReasoningTokens != 12 ||
		usage.OutputTokensDetails.SafetyTokens == nil || *usage.OutputTokensDetails.SafetyTokens != 3 {
		t.Errorf("output details = %+v", usage.OutputTokensDetails)
	}
	if usage.ServerToolUse == nil || usage.ServerToolUse.ToolSearch != 1 ||
		usage.ServerToolUse.ToolFetch != 2 || usage.ServerToolUse.ToolImageSearch == nil ||
		*usage.ServerToolUse.ToolImageSearch != 1 {
		t.Errorf("server tool use = %+v", usage.ServerToolUse)
	}

	if resp.Store == nil || *resp.Store {
		t.Errorf("store = %v", resp.Store)
	}
	if resp.Reasoning == nil || resp.Reasoning.Effort != responses.EffortHigh {
		t.Errorf("reasoning = %+v", resp.Reasoning)
	}
	if resp.Text == nil || resp.Text.Format == nil || resp.Text.Format.Type != responses.FormatJSONSchema {
		t.Errorf("text = %+v", resp.Text)
	}
	if resp.ToolChoice == nil || resp.ToolChoice.Function != "get_weather" {
		t.Errorf("tool choice = %+v", resp.ToolChoice)
	}
	if len(resp.Tools) != 2 || resp.Tools[0].Name != "get_weather" || resp.Tools[1].Type != responses.ToolWebSearch {
		t.Errorf("tools = %+v", resp.Tools)
	}
	if resp.Metadata == nil {
		t.Error("metadata is nil")
	}
}

func TestInputUnion(t *testing.T) {
	marshalCases := []struct {
		name string
		in   responses.Input
		want string
	}{
		{name: "string", in: responses.Input{Text: "hi"}, want: `"hi"`},
		{name: "items", in: responses.Input{Items: []responses.InputItem{}}, want: `[]`},
		{name: "null", in: responses.Input{Null: true}, want: `null`},
		{name: "items win over text", in: responses.Input{Text: "hi", Items: []responses.InputItem{}}, want: `[]`},
	}
	for _, tc := range marshalCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("Marshal = %s, want %s", got, tc.want)
			}
		})
	}

	var text responses.Input
	if err := json.Unmarshal([]byte(`"hello"`), &text); err != nil {
		t.Fatalf("Unmarshal string: %v", err)
	}
	if text.Text != "hello" || text.Items != nil || text.Null {
		t.Errorf("string input = %+v", text)
	}

	var items responses.Input
	if err := json.Unmarshal([]byte(`[{"type":"message","role":"user","content":"hi"}]`), &items); err != nil {
		t.Fatalf("Unmarshal items: %v", err)
	}
	if len(items.Items) != 1 || items.Items[0].Role != responses.RoleUser {
		t.Fatalf("item input = %+v", items)
	}
	if items.Items[0].Content == nil || items.Items[0].Content.Text != "hi" {
		t.Errorf("item content = %+v", items.Items[0].Content)
	}

	var null responses.Input
	if err := json.Unmarshal([]byte(`null`), &null); err != nil {
		t.Fatalf("Unmarshal null: %v", err)
	}
	if !null.Null {
		t.Error("null input not marked")
	}
}

func TestRequestValidation(t *testing.T) {
	imageBoth := &responses.Input{Items: []responses.InputItem{{
		Type: responses.ItemMessage,
		Role: responses.RoleUser,
		Content: new(responses.PartsContent(responses.ContentPart{
			Type:     responses.PartInputImage,
			ImageURL: "https://example.com/a.png",
			FileID:   "file-api-a",
		})),
	}}}
	imageNeither := &responses.Input{Items: []responses.InputItem{{
		Type:    responses.ItemMessage,
		Role:    responses.RoleUser,
		Content: new(responses.PartsContent(responses.ContentPart{Type: responses.PartInputImage})),
	}}}
	emptyFile := &responses.Input{Items: []responses.InputItem{{
		Type:    responses.ItemMessage,
		Role:    responses.RoleUser,
		Content: new(responses.PartsContent(responses.InputFilePart(""))),
	}}}

	valid := func(mutate func(*responses.Request)) *responses.Request {
		req := &responses.Request{Model: "deepseek-flash", Input: responses.StringInput("hi")}
		if mutate != nil {
			mutate(req)
		}
		return req
	}

	cases := []struct {
		name  string
		req   *responses.Request
		field string
	}{
		{name: "nil request", req: nil, field: "request"},
		{name: "missing model", req: &responses.Request{Input: responses.StringInput("hi")}, field: "model"},
		{name: "missing input", req: &responses.Request{Model: "deepseek-flash"}, field: "input"},
		{name: "empty input", req: &responses.Request{Model: "deepseek-flash", Input: responses.StringInput("")}, field: "input"},
		{name: "null input", req: &responses.Request{Model: "deepseek-flash", Input: responses.NullInput()}, field: "input"},
		{name: "empty instructions", req: &responses.Request{Model: "deepseek-flash", Instructions: new("")}, field: "input"},
		{name: "zero max output", req: valid(func(r *responses.Request) { r.MaxOutputTokens = new(0) }), field: "max_output_tokens"},
		{name: "temperature", req: valid(func(r *responses.Request) { r.Temperature = new(2.5) }), field: "temperature"},
		{name: "top_p", req: valid(func(r *responses.Request) { r.TopP = new(1.5) }), field: "top_p"},
		{name: "top_logprobs", req: valid(func(r *responses.Request) { r.TopLogprobs = new(21) }), field: "top_logprobs"},
		{name: "empty tool choice", req: valid(func(r *responses.Request) { r.ToolChoice = &responses.ToolChoice{} }), field: "tool_choice"},
		{name: "mixed tool choice", req: valid(func(r *responses.Request) {
			r.ToolChoice = &responses.ToolChoice{Mode: responses.ToolChoiceAuto, Function: "f"}
		}), field: "tool_choice"},
		{name: "format without type", req: valid(func(r *responses.Request) {
			r.Text = &responses.Text{Format: &responses.TextFormat{}}
		}), field: "text.format.type"},
		{name: "schema without name", req: valid(func(r *responses.Request) {
			r.Text = &responses.Text{Format: &responses.TextFormat{Type: responses.FormatJSONSchema, Schema: map[string]any{}}}
		}), field: "text.format.name"},
		{name: "schema without schema", req: valid(func(r *responses.Request) {
			r.Text = &responses.Text{Format: &responses.TextFormat{Type: responses.FormatJSONSchema, Name: "x"}}
		}), field: "text.format.schema"},
		{name: "function tool without name", req: valid(func(r *responses.Request) {
			r.Tools = []responses.Tool{{Type: responses.ToolFunction}}
		}), field: "tools[0].name"},
		{name: "function tool bad name", req: valid(func(r *responses.Request) {
			r.Tools = []responses.Tool{responses.FunctionTool("bad name", "", nil)}
		}), field: "tools[0].name"},
		{name: "function tool long name", req: valid(func(r *responses.Request) {
			r.Tools = []responses.Tool{responses.FunctionTool(strings.Repeat("a", 129), "", nil)}
		}), field: "tools[0].name"},
		{name: "duplicate tool", req: valid(func(r *responses.Request) {
			r.Tools = []responses.Tool{responses.FunctionTool("f", "", nil), responses.FunctionTool("f", "", nil)}
		}), field: "tools[1].name"},
		{name: "custom tool name", req: valid(func(r *responses.Request) {
			r.Tools = []responses.Tool{responses.CustomTool("other")}
		}), field: "tools[0].name"},
		{name: "tool without type", req: valid(func(r *responses.Request) {
			r.Tools = []responses.Tool{{Name: "f"}}
		}), field: "tools[0].type"},
		{name: "safety without level", req: valid(func(r *responses.Request) {
			r.SafetyCheck = &responses.SafetyCheck{}
		}), field: "safety_check.level"},
		{name: "image both sources", req: valid(func(r *responses.Request) { r.Input = imageBoth }), field: "input[0].content"},
		{name: "image no source", req: valid(func(r *responses.Request) { r.Input = imageNeither }), field: "input[0].content"},
		{name: "file without id", req: valid(func(r *responses.Request) { r.Input = emptyFile }), field: "input[0].content"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if err == nil {
				t.Fatal("Validate returned nil")
			}
			var verr *deepseek.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("error type = %T, want *deepseek.ValidationError", err)
			}
			if !strings.HasPrefix(verr.Field, tc.field) {
				t.Errorf("field = %q, want prefix %q", verr.Field, tc.field)
			}
		})
	}

	validCases := []struct {
		name string
		req  *responses.Request
	}{
		{name: "input only", req: valid(nil)},
		{name: "instructions only", req: &responses.Request{Model: "deepseek-flash", Instructions: new("hi")}},
		{name: "null input with instructions", req: &responses.Request{Model: "deepseek-flash", Input: responses.NullInput(), Instructions: new("hi")}},
		{name: "empty item list", req: &responses.Request{Model: "deepseek-flash", Input: responses.ItemsInput()}},
		{name: "modes", req: valid(func(r *responses.Request) { r.ToolChoice = responses.AutoToolChoice() })},
		{name: "zero top_logprobs", req: valid(func(r *responses.Request) { r.TopLogprobs = new(0) })},
	}
	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.req.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

func TestToolEncoding(t *testing.T) {
	tools := []responses.Tool{
		responses.FunctionTool("get_weather", "Get weather", map[string]any{"type": "object"}),
		responses.CustomTool("apply_patch"),
		responses.WebSearchTool(),
		responses.DeepSeekWebSearchTool(),
		responses.DeepSeekReadURLTool(),
	}
	raw, err := json.Marshal(tools)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `[{"type":"function","name":"get_weather","description":"Get weather","parameters":{"type":"object"}},` +
		`{"type":"custom","name":"apply_patch"},` +
		`{"type":"web_search"},` +
		`{"type":"deepseek_web_search"},` +
		`{"type":"deepseek_read_url"}]`
	if string(raw) != want {
		t.Errorf("tools = %s\nwant  %s", raw, want)
	}
}

func TestRequestDocExampleEncoding(t *testing.T) {
	req := &responses.Request{
		Model:           "deepseek-flash",
		Input:           responses.StringInput("Say hello."),
		Reasoning:       &responses.Reasoning{Effort: responses.EffortNone},
		MaxOutputTokens: new(32),
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"model":"deepseek-flash","input":"Say hello.","reasoning":{"effort":"none"},"max_output_tokens":32}`
	if string(raw) != want {
		t.Errorf("request = %s\nwant      %s", raw, want)
	}
}

func TestResponseDocExample(t *testing.T) {
	const doc = `{
	  "id": "387b53c1-7ad2-4259-9298-61b139f5bd20",
	  "object": "response",
	  "created_at": 1793084842,
	  "status": "completed",
	  "background": false,
	  "completed_at": 1793084843,
	  "content_filters": null,
	  "error": null,
	  "frequency_penalty": 0.0,
	  "incomplete_details": null,
	  "instructions": null,
	  "max_output_tokens": 32,
	  "max_tool_calls": null,
	  "model": "deepseek-flash",
	  "moderation": null,
	  "output": [
	    {
	      "type": "message",
	      "id": "4e706f35-6ea2-468a-bd19-a9c1adc54726",
	      "status": "completed",
	      "content": [
	        {
	          "type": "output_text",
	          "annotations": [],
	          "logprobs": [],
	          "text": "Hello! 👋 How can I help you today?"
	        }
	      ],
	      "phase": "final_answer",
	      "role": "assistant"
	    }
	  ],
	  "parallel_tool_calls": true,
	  "presence_penalty": 0.0,
	  "previous_response_id": null,
	  "prompt_cache_key": null,
	  "prompt_cache_retention": null,
	  "reasoning": {"effort": "none", "summary": null},
	  "safety_identifier": null,
	  "service_tier": "default",
	  "store": false,
	  "temperature": 1.0,
	  "text": {"format": {"type": "text"}, "verbosity": null},
	  "tool_choice": "auto",
	  "tools": [],
	  "top_logprobs": 0,
	  "top_p": 1.0,
	  "truncation": "disabled",
	  "usage": {
	    "input_tokens": 7,
	    "input_tokens_details": {"cached_tokens": 0},
	    "output_tokens": 11,
	    "output_tokens_details": {"reasoning_tokens": 0},
	    "total_tokens": 18
	  },
	  "user": null,
	  "metadata": {}
	}`
	var resp responses.Response
	if err := json.Unmarshal([]byte(doc), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.ID != "387b53c1-7ad2-4259-9298-61b139f5bd20" || resp.Status != responses.StatusCompleted {
		t.Errorf("response = %+v", resp)
	}
	if resp.OutputText() != "Hello! 👋 How can I help you today?" {
		t.Errorf("output text = %q", resp.OutputText())
	}
	if resp.ToolChoice == nil || resp.ToolChoice.Mode != responses.ToolChoiceAuto || resp.ToolChoice.Function != "" {
		t.Errorf("tool choice = %+v", resp.ToolChoice)
	}
	if resp.Text == nil || resp.Text.Format == nil || resp.Text.Format.Type != responses.FormatText {
		t.Errorf("text = %+v", resp.Text)
	}
	if resp.Reasoning == nil || resp.Reasoning.Effort != responses.EffortNone {
		t.Errorf("reasoning = %+v", resp.Reasoning)
	}
	if resp.Store == nil || *resp.Store {
		t.Errorf("store = %v", resp.Store)
	}
	if resp.Usage == nil || resp.Usage.InputTokens != 7 || resp.Usage.OutputTokens != 11 || resp.Usage.TotalTokens != 18 {
		t.Errorf("usage = %+v", resp.Usage)
	}
	if resp.Instructions != nil || resp.Error != nil || resp.IncompleteDetails != nil {
		t.Errorf("nullable fields = %v %v %v", resp.Instructions, resp.Error, resp.IncompleteDetails)
	}
	if resp.Metadata == nil {
		t.Error("metadata is nil")
	}
}

func TestResponseFailed(t *testing.T) {
	const failed = `{"id":"resp_f","object":"response","created_at":3,"status":"failed","error":{"code":"server_error","message":"boom"},"model":"deepseek-flash","output":[]}`
	var resp responses.Response
	if err := json.Unmarshal([]byte(failed), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.Status != responses.StatusFailed {
		t.Errorf("status = %q", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "server_error" || resp.Error.Message != "boom" {
		t.Errorf("error = %+v", resp.Error)
	}
}
