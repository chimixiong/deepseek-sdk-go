package messages

import (
	"encoding/json"
	"testing"
)

const responseFixture = `{
  "id": "msg_01",
  "type": "message",
  "role": "assistant",
  "model": "deepseek-flash",
  "content": [
    {"type": "thinking", "thinking": "I will call the tool.", "signature": "sig-abc"},
    {"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": {"city": "Paris"}},
    {"type": "text", "text": "Checking the weather.", "citations": [
      {"type": "web_search_result_location", "url": "https://example.com/weather", "title": "Weather", "encrypted_index": "", "cited_text": "Checking"}
    ]}
  ],
  "stop_reason": "tool_use",
  "stop_sequence": null,
  "title": "Weather",
  "usage": {
    "input_tokens": 12,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 4,
    "output_tokens": 30,
    "output_tokens_details": {"thinking_tokens": 10, "safety_tokens": 0},
    "service_tier": "standard",
    "server_tool_use": {"web_search_requests": 1, "tool_search": 1, "web_fetch_requests": 0, "tool_fetch": 0, "tool_image_search": 0}
  }
}`

func TestResponseDecode(t *testing.T) {
	var resp Response
	if err := json.Unmarshal([]byte(responseFixture), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.ID != "msg_01" || resp.Type != MessageType || resp.Role != RoleAssistant || resp.Model != "deepseek-flash" {
		t.Fatalf("message header = %#v", resp)
	}
	if len(resp.Content) != 3 {
		t.Fatalf("content = %d blocks, want 3", len(resp.Content))
	}

	thinking := resp.Content[0]
	if thinking.Type != BlockThinking || thinking.Thinking != "I will call the tool." {
		t.Fatalf("thinking = %#v", thinking)
	}
	if thinking.Signature == nil || *thinking.Signature != "sig-abc" {
		t.Fatalf("thinking signature = %#v", thinking.Signature)
	}

	tool := resp.Content[1]
	if tool.Type != BlockToolUse || tool.ID != "toolu_1" || tool.Name != "get_weather" {
		t.Fatalf("tool_use = %#v", tool)
	}
	var input map[string]any
	if err := json.Unmarshal(tool.Input, &input); err != nil {
		t.Fatalf("tool input %s: %v", tool.Input, err)
	}
	if input["city"] != "Paris" {
		t.Fatalf("tool input = %#v", input)
	}

	text := resp.Content[2]
	if text.Type != BlockText || text.Text != "Checking the weather." {
		t.Fatalf("text = %#v", text)
	}
	if len(text.Citations) != 1 {
		t.Fatalf("citations = %#v", text.Citations)
	}
	citation := text.Citations[0]
	if citation.Type != CitationWebSearchResult || citation.URL != "https://example.com/weather" || citation.Title == nil || *citation.Title != "Weather" {
		t.Fatalf("citation = %#v", citation)
	}
	if citation.EncryptedIndex != "" || citation.CitedText != "Checking" {
		t.Fatalf("citation = %#v", citation)
	}

	if resp.StopReason == nil || *resp.StopReason != StopReasonToolUse {
		t.Fatalf("stop_reason = %#v", resp.StopReason)
	}
	if resp.StopSequence != nil {
		t.Fatalf("stop_sequence = %#v", resp.StopSequence)
	}
	if resp.Title == nil || *resp.Title != "Weather" {
		t.Fatalf("title = %#v", resp.Title)
	}

	usage := resp.Usage
	if usage.InputTokens != 12 || usage.CacheCreationInputTokens != 0 || usage.CacheReadInputTokens != 4 || usage.OutputTokens != 30 {
		t.Fatalf("usage = %#v", usage)
	}
	if usage.ServiceTier != ServiceTierStandard {
		t.Fatalf("service_tier = %q", usage.ServiceTier)
	}
	if usage.OutputTokensDetails == nil || usage.OutputTokensDetails.ThinkingTokens != 10 {
		t.Fatalf("output_tokens_details = %#v", usage.OutputTokensDetails)
	}
	server := usage.ServerToolUse
	if server == nil || server.WebSearchRequests != 1 || server.ToolSearch != 1 {
		t.Fatalf("server_tool_use = %#v", server)
	}
	if server.WebFetchRequests == nil || *server.WebFetchRequests != 0 || server.ToolFetch == nil || *server.ToolFetch != 0 || server.ToolImageSearch == nil || *server.ToolImageSearch != 0 {
		t.Fatalf("server_tool_use = %#v", server)
	}
}

func TestResponseRefusal(t *testing.T) {
	const fixture = `{
	  "id": "msg_02", "type": "message", "role": "assistant", "model": "deepseek-flash",
	  "content": [{"type": "text", "text": "I cannot help with that."}],
	  "stop_reason": "refusal",
	  "stop_sequence": null,
	  "stop_details": {"type": "refusal", "category": null, "explanation": "image blocked"},
	  "usage": {"input_tokens": 5, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 7, "service_tier": "standard"}
	}`
	var resp Response
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.StopReason == nil || *resp.StopReason != StopReasonRefusal {
		t.Fatalf("stop_reason = %#v", resp.StopReason)
	}
	if resp.StopDetails == nil || resp.StopDetails.Type != RefusalStopDetailsType {
		t.Fatalf("stop_details = %#v", resp.StopDetails)
	}
	if resp.StopDetails.Category != nil {
		t.Fatalf("category = %#v, want nil", resp.StopDetails.Category)
	}
	if resp.StopDetails.Explanation == nil || *resp.StopDetails.Explanation != "image blocked" {
		t.Fatalf("explanation = %#v", resp.StopDetails.Explanation)
	}
}

func TestResponseServerToolBlocks(t *testing.T) {
	const fixture = `{
	  "id": "msg_03", "type": "message", "role": "assistant", "model": "deepseek-flash",
	  "content": [
	    {
	      "type": "server_tool_use",
	      "id": "srvtoolu_s_00_1",
	      "name": "deepseek_web_search",
	      "input": {"action": "search", "queries": ["Iceland population"]},
	      "caller": {"type": "direct"},
	      "kind": "search"
	    },
	    {
	      "type": "web_search_tool_result",
	      "tool_use_id": "srvtoolu_s_00_1",
	      "content": [
	        {
	          "type": "web_search_result",
	          "title": "Population",
	          "url": "https://example.com/population",
	          "encrypted_content": "enc",
	          "page_age": "2025-10-15T16:00:00+00:00",
	          "snippet": "About 400,000",
	          "site_name": "Example",
	          "site_icon": "https://cdn.example.com/icon.png",
	          "image_url": "https://example.com/image.png"
	        }
	      ],
	      "kind": "search"
	    },
	    {
	      "type": "web_search_tool_result",
	      "tool_use_id": "srvtoolu_s_00_2",
	      "content": [{"type": "web_search_tool_result_error", "error_code": "unavailable"}],
	      "kind": "search"
	    },
	    {
	      "type": "web_fetch_tool_result",
	      "tool_use_id": "srvtoolu_rl_01",
	      "content": {
	        "type": "web_fetch_result",
	        "url": "https://example.com/a",
	        "content": {
	          "type": "document",
	          "source": {"type": "text", "media_type": "text/plain", "data": ""},
	          "title": "Page title"
	        },
	        "published_at": "2026-01-01"
	      },
	      "kind": "read_link",
	      "signature": "q3J0b1RmZ1c4cE1x"
	    },
	    {
	      "type": "web_fetch_tool_result",
	      "tool_use_id": "srvtoolu_rl_02",
	      "content": {"type": "web_fetch_tool_result_error", "error_code": "url_not_allowed"},
	      "kind": "read_link"
	    }
	  ],
	  "stop_reason": "end_turn",
	  "stop_sequence": null,
	  "usage": {
	    "input_tokens": 2912, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 256, "output_tokens": 52,
	    "service_tier": "standard",
	    "server_tool_use": {"web_search_requests": 1, "tool_search": 1}
	  }
	}`
	var resp Response
	if err := json.Unmarshal([]byte(fixture), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(resp.Content) != 5 {
		t.Fatalf("content = %d blocks, want 5", len(resp.Content))
	}

	call := resp.Content[0]
	if call.Type != BlockServerToolUse || call.Name != "deepseek_web_search" || call.Kind != KindSearch {
		t.Fatalf("server_tool_use = %#v", call)
	}
	if call.Caller == nil || call.Caller.Type != CallerDirect {
		t.Fatalf("caller = %#v", call.Caller)
	}
	var callInput map[string]any
	if err := json.Unmarshal(call.Input, &callInput); err != nil {
		t.Fatalf("call input: %v", err)
	}
	if callInput["action"] != "search" {
		t.Fatalf("call input = %#v", callInput)
	}

	search := resp.Content[1]
	if search.Type != BlockWebSearchToolResult || search.Kind != KindSearch || len(search.SearchResults) != 1 {
		t.Fatalf("web_search_tool_result = %#v", search)
	}
	result := search.SearchResults[0]
	if result.Type != SearchResultWebSearch || result.URL != "https://example.com/population" || result.EncryptedContent != "enc" {
		t.Fatalf("search result = %#v", result)
	}
	if result.PageAge == nil || *result.PageAge != "2025-10-15T16:00:00+00:00" || result.SiteName == nil || *result.SiteName != "Example" {
		t.Fatalf("search result = %#v", result)
	}
	if result.ImageURL == nil || *result.ImageURL != "https://example.com/image.png" {
		t.Fatalf("image_url = %#v", result.ImageURL)
	}

	errResult := resp.Content[2].SearchResults
	if len(errResult) != 1 || errResult[0].Type != SearchResultError || errResult[0].ErrorCode != "unavailable" {
		t.Fatalf("search error = %#v", errResult)
	}

	fetch := resp.Content[3]
	if fetch.Type != BlockWebFetchToolResult || fetch.Kind != KindReadLink || fetch.Signature == nil || *fetch.Signature != "q3J0b1RmZ1c4cE1x" {
		t.Fatalf("web_fetch_tool_result = %#v", fetch)
	}
	if fetch.Fetch == nil || fetch.Fetch.Type != WebFetchResultPage || fetch.Fetch.URL != "https://example.com/a" {
		t.Fatalf("fetch result = %#v", fetch.Fetch)
	}
	if fetch.Fetch.Content == nil || fetch.Fetch.Content.Source == nil || fetch.Fetch.Content.Source.Type != SourceText || fetch.Fetch.Content.Source.Data != "" {
		t.Fatalf("fetch document = %#v", fetch.Fetch.Content)
	}
	if fetch.Fetch.PublishedAt == nil || *fetch.Fetch.PublishedAt != "2026-01-01" {
		t.Fatalf("published_at = %#v", fetch.Fetch.PublishedAt)
	}

	fetchErr := resp.Content[4].Fetch
	if fetchErr == nil || fetchErr.Type != WebFetchResultError || fetchErr.ErrorCode != "url_not_allowed" {
		t.Fatalf("fetch error = %#v", fetchErr)
	}

	if _, err := json.Marshal(resp.Content); err != nil {
		t.Fatalf("re-marshal blocks: %v", err)
	}
}
