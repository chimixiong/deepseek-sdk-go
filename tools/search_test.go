package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const searchFixture = `{
  "results": [
    {
      "rerank_score": 2.4,
      "title": "DeepSeek V4 model released",
      "url": "https://example.com/deepseek-v4",
      "snippet": "DeepSeek V4 is now available, with tool calling and long-context support.",
      "summary": "DeepSeek has released the V4 model. The release adds improved tool calling and long-context support for agent workflows.",
      "content": "",
      "site_name": "Example News",
      "site_icon": "https://example.com/favicon.ico",
      "date": "2026-06-01T00:00:00+00:00",
      "authority": 3
    }
  ]
}`

func TestSearch(t *testing.T) {
	var (
		gotPath   string
		gotAuth   string
		gotMethod string
		gotBody   map[string]any
	)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		io.WriteString(w, searchFixture)
	})

	count, oriQuestion, localization, needContent := 1, "which DeepSeek model was released?", "1.2.3.4", true
	resp, err := c.Search(context.Background(), &SearchRequest{
		Query:        "DeepSeek API documentation",
		Count:        &count,
		OriQuestion:  &oriQuestion,
		Localization: &localization,
		NeedContent:  &needContent,
		Extra:        map[string]json.RawMessage{"custom_flag": json.RawMessage(`true`)},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if gotMethod != http.MethodPost || gotPath != "/v1/tools/search" {
		t.Fatalf("request = %s %s, want POST /v1/tools/search", gotMethod, gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotBody["query"] != "DeepSeek API documentation" {
		t.Fatalf("query = %v", gotBody["query"])
	}
	if gotBody["count"] != float64(1) {
		t.Fatalf("count = %v", gotBody["count"])
	}
	if gotBody["ori_question"] != oriQuestion {
		t.Fatalf("ori_question = %v", gotBody["ori_question"])
	}
	if gotBody["localization"] != localization {
		t.Fatalf("localization = %v", gotBody["localization"])
	}
	if gotBody["need_content"] != true {
		t.Fatalf("need_content = %v", gotBody["need_content"])
	}
	if gotBody["custom_flag"] != true {
		t.Fatalf("custom_flag = %v, want true", gotBody["custom_flag"])
	}

	if len(resp.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(resp.Results))
	}
	result := resp.Results[0]
	if result.RerankScore != 2.4 ||
		result.Title != "DeepSeek V4 model released" ||
		result.URL != "https://example.com/deepseek-v4" ||
		result.Snippet != "DeepSeek V4 is now available, with tool calling and long-context support." ||
		result.Summary != "DeepSeek has released the V4 model. The release adds improved tool calling and long-context support for agent workflows." ||
		result.Content != "" ||
		result.Authority != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.SiteName == nil || *result.SiteName != "Example News" {
		t.Fatalf("site_name = %v", result.SiteName)
	}
	if result.SiteIcon == nil || *result.SiteIcon != "https://example.com/favicon.ico" {
		t.Fatalf("site_icon = %v", result.SiteIcon)
	}
	if result.Date == nil || *result.Date != "2026-06-01T00:00:00+00:00" {
		t.Fatalf("date = %v", result.Date)
	}
}

func TestSearchNullableFields(t *testing.T) {
	const fixture = `{"results":[{"rerank_score":1.5,"title":"t","url":"https://example.com","snippet":"s","summary":"m","content":"","site_name":null,"site_icon":null,"date":null,"authority":-1}]}`
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, fixture)
	})
	resp, err := c.Search(context.Background(), &SearchRequest{Query: "q"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	result := resp.Results[0]
	if result.SiteName != nil || result.SiteIcon != nil || result.Date != nil {
		t.Fatalf("nullable fields = %v/%v/%v, want nil", result.SiteName, result.SiteIcon, result.Date)
	}
	if result.Authority != -1 {
		t.Fatalf("authority = %d, want -1", result.Authority)
	}
}

func TestSearchBoundaries(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if body["count"] != float64(50) {
			t.Errorf("count = %v, want 50", body["count"])
		}
		io.WriteString(w, `{"results":[]}`)
	})
	query := strings.Repeat("a", 128)
	count := 50
	resp, err := c.Search(context.Background(), &SearchRequest{Query: query, Count: &count})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("results = %d, want 0", len(resp.Results))
	}
}

func TestSearchValidation(t *testing.T) {
	c := newTestClient(t, noRequest(t))
	long := strings.Repeat("a", 129)
	tests := []struct {
		name  string
		req   *SearchRequest
		field string
	}{
		{"nil request", nil, "request"},
		{"empty query", &SearchRequest{}, "query"},
		{"whitespace query", &SearchRequest{Query: " \t "}, "query"},
		{"query too long", &SearchRequest{Query: long}, "query"},
		{"count too small", &SearchRequest{Query: "q", Count: new(0)}, "count"},
		{"count too large", &SearchRequest{Query: "q", Count: new(51)}, "count"},
		{"empty ori_question", &SearchRequest{Query: "q", OriQuestion: new("")}, "ori_question"},
		{"ori_question too long", &SearchRequest{Query: "q", OriQuestion: new(long)}, "ori_question"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Search(context.Background(), tt.req)
			requireValidationError(t, err, tt.field)
		})
	}
}

func TestSearchAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"message":"query is invalid","type":"invalid_request_error","param":"query"}}`)
	})
	_, err := c.Search(context.Background(), &SearchRequest{Query: "q"})
	apiErr := requireAPIError(t, err, http.StatusBadRequest)
	if apiErr.Message != "query is invalid" || apiErr.Param != "query" || apiErr.Type != "invalid_request_error" {
		t.Fatalf("unexpected error: %+v", apiErr)
	}
}

func TestNilClient(t *testing.T) {
	var c *Client
	_, err := c.Search(context.Background(), &SearchRequest{Query: "q"})
	if err == nil || err.Error() != "tools: nil deepseek client" {
		t.Fatalf("error = %v, want nil client error", err)
	}
	if c.DeepSeek() != nil {
		t.Fatal("DeepSeek() on nil client must be nil")
	}
}

func TestSearchExtraCannotOverrideCount(t *testing.T) {
	c := newTestClient(t, noRequest(t))
	_, err := c.Search(context.Background(), &SearchRequest{
		Query: "q",
		Extra: map[string]json.RawMessage{"count": json.RawMessage(`999`)},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicates a modelled field") {
		t.Fatalf("err = %v, want the Extra override to be rejected", err)
	}
}
