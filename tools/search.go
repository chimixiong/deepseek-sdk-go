package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/chimixiong/deepseek-sdk-go"
)

const (
	maxSearchQueryChars = 128
	maxSearchCount      = 50
)

// SearchRequest is the body of POST /v1/tools/search.
type SearchRequest struct {
	// Query is the search expression. The API supports site:, -site: and
	// quoted phrases. It must be 1-128 Unicode characters and not all
	// whitespace.
	Query string `json:"query"`
	// Count is the number of results, 1-50. The server defaults to 10.
	Count *int `json:"count,omitempty"`
	// OriQuestion is the user question that triggered the search, 1-128
	// characters. nil is omitted, which the API treats like null.
	OriQuestion *string `json:"ori_question,omitempty"`
	// Localization is an IP address used to prefer local results. The server
	// defaults to the empty string.
	Localization *string `json:"localization,omitempty"`
	// NeedContent asks the API to fill SearchResult.Content with page bodies.
	// It defaults to false, in which case Content is always empty.
	NeedContent *bool `json:"need_content,omitempty"`
	// Extra adds top-level fields the SDK does not model.
	Extra map[string]json.RawMessage `json:"-"`
}

// Validate reports the documented request constraints.
func (r *SearchRequest) Validate() error {
	if r == nil {
		return deepseek.Invalid("request", "must not be nil")
	}
	switch n := utf8.RuneCountInString(r.Query); {
	case n == 0:
		return deepseek.Invalid("query", "must not be empty")
	case n > maxSearchQueryChars:
		return deepseek.Invalid("query", "must be at most %d characters, got %d", maxSearchQueryChars, n)
	case strings.TrimSpace(r.Query) == "":
		return deepseek.Invalid("query", "must not be all whitespace")
	}
	if r.Count != nil && (*r.Count < 1 || *r.Count > maxSearchCount) {
		return deepseek.Invalid("count", "must be between 1 and %d, got %d", maxSearchCount, *r.Count)
	}
	if r.OriQuestion != nil {
		switch n := utf8.RuneCountInString(*r.OriQuestion); {
		case n == 0:
			return deepseek.Invalid("ori_question", "must not be empty when provided")
		case n > maxSearchQueryChars:
			return deepseek.Invalid("ori_question", "must be at most %d characters, got %d", maxSearchQueryChars, n)
		}
	}
	return nil
}

// MarshalJSON adds Extra fields to the encoded request.
func (r *SearchRequest) MarshalJSON() ([]byte, error) {
	type plain SearchRequest
	return deepseek.MarshalWithExtra((*plain)(r), r.Extra)
}

// SearchResponse is the response of POST /v1/tools/search.
type SearchResponse struct {
	// Results is empty when nothing matched.
	Results []SearchResult `json:"results"`
}

// SearchResult is one ranked web page.
type SearchResult struct {
	// RerankScore is 0-3; 1.5 and above marks a high-quality result.
	RerankScore float64 `json:"rerank_score"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	// Snippet is a short query-relevant excerpt, usually 100-200 characters.
	Snippet string `json:"snippet"`
	// Summary is a long query-relevant summary, usually 600-800 characters.
	Summary string `json:"summary"`
	// Content is the page body; empty unless the request set NeedContent.
	Content string `json:"content"`
	// SiteName is nil when the API reports null.
	SiteName *string `json:"site_name"`
	// SiteIcon is nil when the API reports null.
	SiteIcon *string `json:"site_icon"`
	// Date is the RFC 3339 publication time in UTC, or nil when unknown.
	Date *string `json:"date"`
	// Authority is 0-4, or -1 when unknown.
	Authority int `json:"authority"`
}

// Search searches the web.
func (c *Client) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	client, err := c.transport()
	if err != nil {
		return nil, err
	}
	var out SearchResponse
	err = client.DoJSON(ctx, &deepseek.Request{
		Method: http.MethodPost,
		URL:    client.ToolURL("search"),
		JSON:   req,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
