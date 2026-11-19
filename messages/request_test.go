package messages

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	deepseek "github.com/chimixiong/deepseek-sdk-go"
)

func validRequest() *Request {
	return &Request{
		Model:     "deepseek-flash",
		Messages:  []Message{{Role: RoleUser, Content: Content{Text: "Hi"}}},
		MaxTokens: new(64),
	}
}

func TestRequestMarshal(t *testing.T) {
	req := &Request{
		Model: "deepseek-flash",
		Messages: []Message{
			{Role: RoleUser, Content: Content{Text: "Weather in Paris?"}},
			{Role: RoleAssistant, Content: Content{Blocks: []Block{
				{Type: BlockThinking, Thinking: "Need the tool.", Signature: new("sig-request")},
				{Type: BlockToolUse, ID: "toolu_1", Name: "get_weather", Input: json.RawMessage(`{"city":"Paris"}`)},
			}}},
			{Role: RoleUser, Content: Content{Blocks: []Block{
				{Type: BlockToolResult, ToolUseID: "toolu_1", Content: &Content{Text: "18C"}},
				{Type: BlockImage, Source: &Source{Type: SourceBase64, MediaType: "image/png", Data: "iVBORw0KGgo="}},
				{Type: BlockText, Text: "Summarize."},
			}}},
		},
		System:        &Content{Blocks: []Block{{Type: BlockText, Text: "You are concise."}}},
		MaxTokens:     new(512),
		Thinking:      &Thinking{Type: ThinkingDisabled},
		OutputConfig:  &OutputConfig{Effort: EffortHigh},
		Tools:         []Tool{{Name: "get_weather", Description: new("Weather lookup"), InputSchema: map[string]any{"type": "object"}}},
		ToolChoice:    &ToolChoice{Type: ToolChoiceTool, Name: "get_weather"},
		StopSequences: []string{"\n\n"},
		Temperature:   new(0.5),
		TopP:          new(1.0),
		Metadata:      &Metadata{UserID: new("user-1")},
		SafetyCheck:   &SafetyCheck{Level: SafetyHigh},
		Title:         &Title{Enabled: true},
		Extra:         map[string]json.RawMessage{"future_field": json.RawMessage(`{"a":1}`)},
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	doc := decodeObject(t, data)

	system, ok := doc["system"].([]any)
	if !ok || len(system) != 1 {
		t.Fatalf("system = %#v, want one text block", doc["system"])
	}
	if block := system[0].(map[string]any); block["type"] != "text" || block["text"] != "You are concise." {
		t.Fatalf("system block = %#v", block)
	}

	messages := doc["messages"].([]any)
	if len(messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(messages))
	}
	if got := messages[2].(map[string]any)["content"].([]any); len(got) != 3 {
		t.Fatalf("last content = %#v", got)
	} else {
		wantOrder := []string{"tool_result", "image", "text"}
		for i, want := range wantOrder {
			if got[i].(map[string]any)["type"] != want {
				t.Fatalf("content[%d] type = %v, want %q", i, got[i].(map[string]any)["type"], want)
			}
		}
		if got[0].(map[string]any)["content"] != "18C" {
			t.Fatalf("tool_result content = %#v", got[0])
		}
		source := got[1].(map[string]any)["source"].(map[string]any)
		if source["type"] != "base64" || source["media_type"] != "image/png" || source["data"] != "iVBORw0KGgo=" {
			t.Fatalf("image source = %#v", source)
		}
	}

	assistantContent := messages[1].(map[string]any)["content"].([]any)
	if assistantContent[0].(map[string]any)["signature"] != "sig-request" {
		t.Fatalf("thinking signature = %#v", assistantContent[0])
	}
	input := assistantContent[1].(map[string]any)["input"].(map[string]any)
	if input["city"] != "Paris" {
		t.Fatalf("tool input = %#v", input)
	}

	if doc["tool_choice"].(map[string]any)["name"] != "get_weather" {
		t.Fatalf("tool_choice = %#v", doc["tool_choice"])
	}
	if doc["output_config"].(map[string]any)["effort"] != "high" {
		t.Fatalf("output_config = %#v", doc["output_config"])
	}
	tools := doc["tools"].([]any)
	tool := tools[0].(map[string]any)
	if _, ok := tool["type"]; ok {
		t.Fatalf("client tool must omit type: %#v", tool)
	}
	if tool["input_schema"].(map[string]any)["type"] != "object" {
		t.Fatalf("input_schema = %#v", tool["input_schema"])
	}
	if doc["max_tokens"] != float64(512) || doc["temperature"] != 0.5 || doc["top_p"] != 1.0 {
		t.Fatalf("sampling fields = %#v", doc)
	}
	if _, ok := doc["stream"]; ok {
		t.Fatalf("stream should be omitted when false")
	}
	if doc["title"].(map[string]any)["enabled"] != true {
		t.Fatalf("title = %#v", doc["title"])
	}
	if !reflect.DeepEqual(doc["future_field"], map[string]any{"a": float64(1)}) {
		t.Fatalf("extra = %#v", doc["future_field"])
	}
}

func TestRequestSystemForms(t *testing.T) {
	text := &Request{Model: "m", Messages: []Message{{Role: RoleUser, Content: Content{Text: "hi"}}}, System: &Content{Text: "be brief"}}
	null := &Request{Model: "m", Messages: []Message{{Role: RoleUser, Content: Content{Text: "hi"}}}, System: &Content{Null: true}, ToolChoice: &ToolChoice{Null: true}}
	for _, req := range []*Request{text, null} {
		if err := req.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	}

	data, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeObject(t, data)["system"]; got != "be brief" {
		t.Fatalf("system = %#v, want string", got)
	}

	data, err = json.Marshal(null)
	if err != nil {
		t.Fatal(err)
	}
	doc := decodeObject(t, data)
	system, ok := doc["system"]
	if !ok || system != nil {
		t.Fatalf("system = %#v, want explicit null", system)
	}
	choice, ok := doc["tool_choice"]
	if !ok || choice != nil {
		t.Fatalf("tool_choice = %#v, want explicit null", choice)
	}
}

func TestRequestServerTools(t *testing.T) {
	req := validRequest()
	req.Tools = []Tool{
		{Type: ToolWebSearch20250305, Name: "web_search", MaxUses: new(2)},
		{Type: ToolDeepSeekWebSearch},
		{Type: ToolDeepSeekReadURL, Name: "deepseek_read_url"},
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	tools := decodeObject(t, data)["tools"].([]any)
	search := tools[0].(map[string]any)
	if search["type"] != "web_search_20250305" || search["name"] != "web_search" || search["max_uses"] != float64(2) {
		t.Fatalf("web search tool = %#v", search)
	}
	deep := tools[1].(map[string]any)
	if deep["type"] != "deepseek_web_search" {
		t.Fatalf("deepseek tool = %#v", deep)
	}
	if _, ok := deep["name"]; ok {
		t.Fatalf("optional deepseek name should be omitted: %#v", deep)
	}
}

func TestRequestBlockRoundTrip(t *testing.T) {
	blocks := []Block{
		{Type: BlockText, Text: "hello", Citations: []Citation{{
			Type: CitationWebSearchResult, URL: "https://example.com", Title: new("Example"), CitedText: "hello",
		}}},
		{Type: BlockDocument, Source: &Source{Type: SourceText, MediaType: "text/plain", Data: ""}, Title: new("doc.txt")},
		{Type: BlockToolResult, ToolUseID: "toolu_1", Content: &Content{Blocks: []Block{
			{Type: BlockText, Text: "ok"},
		}}, IsError: new(true)},
		{Type: BlockThinking, Thinking: "why", Signature: new("sig")},
		{Type: BlockToolUse, ID: "toolu_2", Name: "lookup", Input: json.RawMessage(`{"q":"x"}`)},
		{Type: BlockServerToolUse, ID: "srvtoolu_1", Name: "web_search", Input: json.RawMessage(`{"query":"x"}`), Caller: &Caller{Type: CallerDirect}, Kind: KindSearch},
		{Type: BlockWebSearchToolResult, ToolUseID: "srvtoolu_1", SearchResults: []WebSearchResult{
			{Type: SearchResultWebSearch, Title: "T", URL: "https://example.com", EncryptedContent: "enc", Snippet: new("s")},
			{Type: SearchResultError, ErrorCode: "unavailable"},
		}, Kind: KindSearch},
		{Type: BlockWebFetchToolResult, ToolUseID: "srvtoolu_1", Fetch: &WebFetchResult{
			Type: WebFetchResultPage, URL: "https://example.com", Content: &WebFetchDocument{
				Type: "document", Source: &Source{Type: SourceText, MediaType: "text/plain"}, Title: new("Page"),
			},
		}, Kind: KindReadLink, Signature: new("pagesig")},
	}
	for _, block := range blocks {
		data, err := json.Marshal(block)
		if err != nil {
			t.Fatalf("marshal %s: %v", block.Type, err)
		}
		var back Block
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("unmarshal %s: %v (%s)", block.Type, err, data)
		}
		if back.Type != block.Type {
			t.Fatalf("type = %q, want %q", back.Type, block.Type)
		}
		again, err := json.Marshal(back)
		if err != nil {
			t.Fatalf("re-marshal %s: %v", block.Type, err)
		}
		if !reflect.DeepEqual(decodeObject(t, data), decodeObject(t, again)) {
			t.Fatalf("%s round trip:\n  first: %s\n  again: %s", block.Type, data, again)
		}
	}
}

func TestRequestUnknownBlockPreserved(t *testing.T) {
	raw := json.RawMessage(`{"type":"future_block","payload":{"x":1}}`)
	var block Block
	if err := json.Unmarshal(raw, &block); err != nil {
		t.Fatal(err)
	}
	if block.Type != "future_block" || !reflect.DeepEqual(block.Raw, raw) {
		t.Fatalf("block = %#v", block)
	}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodeObject(t, data), decodeObject(t, raw)) {
		t.Fatalf("raw = %s", data)
	}
}

func TestValidation(t *testing.T) {
	clientTool := func(name string) Tool {
		return Tool{Name: name, InputSchema: map[string]any{"type": "object"}}
	}
	toolUse := func(id string) Block {
		return Block{Type: BlockToolUse, ID: id, Name: "lookup", Input: json.RawMessage(`{}`)}
	}
	toolResult := func(id string) Block {
		return Block{Type: BlockToolResult, ToolUseID: id, Content: &Content{Text: "ok"}}
	}
	userBlocks := func(blocks ...Block) []Message {
		return []Message{{Role: RoleUser, Content: Content{Blocks: blocks}}}
	}

	tests := []struct {
		name   string
		field  string
		mutate func(*Request)
	}{
		{"missing model", "model", func(r *Request) { r.Model = "" }},
		{"no messages", "messages", func(r *Request) { r.Messages = nil }},
		{"max tokens zero", "max_tokens", func(r *Request) { r.MaxTokens = new(0) }},
		{"temperature high", "temperature", func(r *Request) { r.Temperature = new(2.5) }},
		{"top p zero", "top_p", func(r *Request) { r.TopP = new(0.0) }},
		{"too many stops", "stop_sequences", func(r *Request) {
			r.StopSequences = make([]string, 17)
			for i := range r.StopSequences {
				r.StopSequences[i] = "x"
			}
		}},
		{"empty stop", "stop_sequences[0]", func(r *Request) { r.StopSequences = []string{""} }},
		{"user id too long", "metadata.user_id", func(r *Request) {
			r.Metadata = &Metadata{UserID: new(strings.Repeat("u", 513))}
		}},
		{"thinking without type", "thinking.type", func(r *Request) { r.Thinking = &Thinking{} }},
		{"negative thinking budget", "thinking.budget_tokens", func(r *Request) {
			r.Thinking = &Thinking{Type: ThinkingEnabled, BudgetTokens: new(-1)}
		}},
		{"safety without level", "safety_check.level", func(r *Request) { r.SafetyCheck = &SafetyCheck{} }},
		{"safety unknown level", "safety_check.level", func(r *Request) {
			r.SafetyCheck = &SafetyCheck{Level: "medium"}
		}},
		{"unknown tool type", "tools[0].type", func(r *Request) {
			r.Tools = []Tool{{Type: "mystery_tool", Name: "x"}}
		}},
		{"system non text", "system[0].type", func(r *Request) {
			r.System = &Content{Blocks: []Block{{Type: BlockImage, Source: &Source{Type: SourceURL, URL: "https://example.com/a.png"}}}}
		}},
		{"client tool without name", "tools[0].name", func(r *Request) {
			r.Tools = []Tool{{InputSchema: map[string]any{"type": "object"}}}
		}},
		{"client tool bad name", "tools[0].name", func(r *Request) { r.Tools = []Tool{clientTool("bad name")} }},
		{"client tool without schema", "tools[0].input_schema", func(r *Request) { r.Tools = []Tool{{Name: "lookup"}} }},
		{"client tool schema type", "tools[0].input_schema.type", func(r *Request) {
			r.Tools = []Tool{{Name: "lookup", InputSchema: map[string]any{"type": "array"}}}
		}},
		{"duplicate tool names", "tools[1].name", func(r *Request) {
			r.Tools = []Tool{clientTool("lookup"), clientTool("lookup")}
		}},
		{"web search name", "tools[0].name", func(r *Request) {
			r.Tools = []Tool{{Type: ToolWebSearch20250305, Name: "search"}}
		}},
		{"negative max uses", "tools[0].max_uses", func(r *Request) {
			r.Tools = []Tool{{Type: ToolWebSearch20250305, Name: "web_search", MaxUses: new(-1)}}
		}},
		{"tool choice without type", "tool_choice.type", func(r *Request) { r.ToolChoice = &ToolChoice{} }},
		{"named tool choice without name", "tool_choice.name", func(r *Request) {
			r.ToolChoice = &ToolChoice{Type: ToolChoiceTool}
		}},
		{"content null", "messages[0].content", func(r *Request) {
			r.Messages = []Message{{Role: RoleUser, Content: Content{Null: true}}}
		}},
		{"empty block array", "messages[0].content", func(r *Request) {
			r.Messages = []Message{{Role: RoleUser, Content: Content{Blocks: []Block{}}}}
		}},
		{"block without type", "messages[0].content[0].type", func(r *Request) {
			r.Messages = userBlocks(Block{})
		}},
		{"thinking in user message", "messages[0].content[0].type", func(r *Request) {
			r.Messages = userBlocks(Block{Type: BlockThinking, Thinking: "why"})
		}},
		{"tool result in assistant message", "messages[0].content[0].type", func(r *Request) {
			r.Messages = []Message{{Role: RoleAssistant, Content: Content{Blocks: []Block{toolResult("toolu_1")}}}}
		}},
		{"invalid role", "messages[0].role", func(r *Request) {
			r.Messages = []Message{{Role: "system", Content: Content{Text: "x"}}}
		}},
		{"latest reminder bad content", "messages[0].content", func(r *Request) {
			r.Messages = []Message{
				{Role: RoleLatestReminder, Content: Content{Text: "2026-11-18,Wednesday"}},
				{Role: RoleUser, Content: Content{Text: "hi"}},
			}
		}},
		{"trailing tool use", "messages[0].content", func(r *Request) {
			r.Messages = []Message{{Role: RoleAssistant, Content: Content{Blocks: []Block{toolUse("toolu_1")}}}}
		}},
		{"image without source", "messages[0].content[0].source", func(r *Request) {
			r.Messages = userBlocks(Block{Type: BlockImage})
		}},
		{"image with text source", "messages[0].content[0].source.type", func(r *Request) {
			r.Messages = userBlocks(Block{Type: BlockImage, Source: &Source{Type: SourceText, Data: "x"}})
		}},
		{"base64 without data", "messages[0].content[0].source.data", func(r *Request) {
			r.Messages = userBlocks(Block{Type: BlockImage, Source: &Source{Type: SourceBase64, MediaType: "image/png"}})
		}},
		{"document with url source", "messages[0].content[0].source.type", func(r *Request) {
			r.Messages = userBlocks(Block{Type: BlockDocument, Source: &Source{Type: SourceURL, URL: "https://example.com"}})
		}},
		{"tool use without id", "messages[1].content[0].id", func(r *Request) {
			r.Messages = []Message{
				{Role: RoleUser, Content: Content{Text: "hi"}},
				{Role: RoleAssistant, Content: Content{Blocks: []Block{{Type: BlockToolUse, Name: "lookup"}}}},
			}
		}},
		{"tool result out of order", "messages[0].content[1].type", func(r *Request) {
			r.Messages = userBlocks(Block{Type: BlockText, Text: "hi"}, toolResult("toolu_1"))
		}},
		{"duplicate tool result", "messages[0].content[1].tool_use_id", func(r *Request) {
			r.Messages = userBlocks(toolResult("toolu_1"), toolResult("toolu_1"))
		}},
		{"orphan tool result", "messages[0].content", func(r *Request) {
			r.Messages = userBlocks(toolResult("toolu_1"))
		}},
		{"missing tool result", "messages[0].content", func(r *Request) {
			r.Messages = []Message{
				{Role: RoleAssistant, Content: Content{Blocks: []Block{toolUse("toolu_1")}}},
				{Role: RoleUser, Content: Content{Text: "never mind"}},
			}
		}},
		{"extra tool result", "messages[1].content", func(r *Request) {
			r.Messages = []Message{
				{Role: RoleAssistant, Content: Content{Blocks: []Block{toolUse("toolu_1")}}},
				{Role: RoleUser, Content: Content{Blocks: []Block{toolResult("toolu_2")}}},
			}
		}},
		{"latest reminder last", "messages[0].role", func(r *Request) {
			r.Messages = []Message{{Role: RoleLatestReminder, Content: Content{Text: "2026-11-16,星期一,Singapore,Web,English"}}}
		}},
		{"latest reminder before assistant", "messages[0].role", func(r *Request) {
			r.Messages = []Message{
				{Role: RoleLatestReminder, Content: Content{Text: "2026-11-16,星期一,Singapore,Web,English"}},
				{Role: RoleAssistant, Content: Content{Text: "hi"}},
			}
		}},
		{"latest reminder with blocks", "messages[0].content", func(r *Request) {
			r.Messages = []Message{
				{Role: RoleLatestReminder, Content: Content{Blocks: []Block{{Type: BlockText, Text: "x"}}}},
				{Role: RoleUser, Content: Content{Text: "hi"}},
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := validRequest()
			test.mutate(req)
			err := req.Validate()
			if err == nil {
				t.Fatalf("Validate returned nil")
			}
			var verr *deepseek.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("error = %T %v, want *deepseek.ValidationError", err, err)
			}
			if verr.Field != test.field {
				t.Fatalf("field = %q, want %q (%v)", verr.Field, test.field, err)
			}
		})
	}
}

func TestValidationAcceptsLatestReminderBeforeUser(t *testing.T) {
	req := validRequest()
	req.Messages = []Message{
		{Role: RoleLatestReminder, Content: Content{Text: "2026-11-16,星期一,Singapore,Web,English"}},
		{Role: RoleUser, Content: Content{Text: "Hi"}},
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateRejectsEmptyContent(t *testing.T) {
	req := validRequest()
	req.Messages = []Message{{Role: RoleUser, Content: Content{Text: ""}}}
	if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("err = %v, want an empty content error", err)
	}
	req.Messages = []Message{{Role: RoleUser, Content: Content{Blocks: []Block{
		{Type: BlockText, Text: ""},
	}}}}
	if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("err = %v, want an empty text block error", err)
	}
}

func decodeObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	return out
}
