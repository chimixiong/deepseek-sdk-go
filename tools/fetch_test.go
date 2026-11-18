package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const fetchFixture = `{
  "title": "首次调用 API | DeepSeek API Docs",
  "content": "# 首次调用 API\n\nDeepSeek API 使用与 OpenAI/Anthropic 兼容的 API 格式，通过修改配置，您可以使用 OpenAI/Anthropic SDK 来访问 DeepSeek API，或使用与 OpenAI/Anthropic API 兼容的软件。"
}`

func TestFetch(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		io.WriteString(w, fetchFixture)
	})

	req := &FetchRequest{URL: "https://api-docs.deepseek.com/zh-cn/"}
	resp, err := c.Fetch(context.Background(), req)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotPath != "/v1/tools/fetch" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["url"] != req.URL {
		t.Fatalf("url = %v", gotBody["url"])
	}
	if resp.Title != "首次调用 API | DeepSeek API Docs" {
		t.Fatalf("title = %q", resp.Title)
	}
	if resp.Content != "# 首次调用 API\n\nDeepSeek API 使用与 OpenAI/Anthropic 兼容的 API 格式，通过修改配置，您可以使用 OpenAI/Anthropic SDK 来访问 DeepSeek API，或使用与 OpenAI/Anthropic API 兼容的软件。" {
		t.Fatalf("content = %q", resp.Content)
	}
}

func TestFetchExtra(t *testing.T) {
	var gotBody map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		io.WriteString(w, `{"title":"","content":""}`)
	})
	_, err := c.Fetch(context.Background(), &FetchRequest{
		URL:   "https://example.com/",
		Extra: map[string]json.RawMessage{"fresh": json.RawMessage(`true`)},
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotBody["fresh"] != true {
		t.Fatalf("fresh = %v, want true", gotBody["fresh"])
	}
}

func TestFetchValidation(t *testing.T) {
	c := newTestClient(t, noRequest(t))
	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"too long", "https://example.com/" + strings.Repeat("a", 8192)},
		{"relative", "/docs"},
		{"ftp", "ftp://example.com/file"},
		{"missing host", "https:///path"},
		{"empty hostname with port", "https://:8080/"},
		{"credentials", "https://user:pass@example.com/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Fetch(context.Background(), &FetchRequest{URL: tt.url})
			requireValidationError(t, err, "url")
		})
	}

	_, err := c.Fetch(context.Background(), nil)
	requireValidationError(t, err, "request")
}
