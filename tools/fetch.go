package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"unicode/utf8"

	"github.com/chimixiong/deepseek-sdk-go"
)

const maxFetchURLLength = 8192

// FetchRequest is the body of POST /v1/tools/fetch.
type FetchRequest struct {
	// URL is an absolute HTTP or HTTPS URL with a host and without user
	// credentials, at most 8192 characters.
	URL string `json:"url"`
	// Extra adds top-level fields the SDK does not model.
	Extra map[string]json.RawMessage `json:"-"`
}

// Validate reports the documented request constraints.
func (r *FetchRequest) Validate() error {
	if r == nil {
		return deepseek.Invalid("request", "must not be nil")
	}
	switch n := utf8.RuneCountInString(r.URL); {
	case n == 0:
		return deepseek.Invalid("url", "must not be empty")
	case n > maxFetchURLLength:
		return deepseek.Invalid("url", "must be at most %d characters, got %d", maxFetchURLLength, n)
	}
	u, err := url.Parse(r.URL)
	if err != nil {
		return deepseek.Invalid("url", "must be a valid URL: %v", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return deepseek.Invalid("url", "must be an absolute HTTP or HTTPS URL with a host")
	}
	if u.User != nil {
		return deepseek.Invalid("url", "must not contain user credentials")
	}
	return nil
}

// MarshalJSON adds Extra fields to the encoded request.
func (r *FetchRequest) MarshalJSON() ([]byte, error) {
	type plain FetchRequest
	return deepseek.MarshalWithExtra((*plain)(r), r.Extra)
}

// FetchResponse is the response of POST /v1/tools/fetch.
type FetchResponse struct {
	// Title is empty when the page has no title.
	Title string `json:"title"`
	// Content is the page body as Markdown.
	Content string `json:"content"`
}

// Fetch downloads a page and returns its title and Markdown content.
func (c *Client) Fetch(ctx context.Context, req *FetchRequest) (*FetchResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	client, err := c.transport()
	if err != nil {
		return nil, err
	}
	var out FetchResponse
	err = client.DoJSON(ctx, &deepseek.Request{
		Method: http.MethodPost,
		URL:    client.ToolURL("fetch"),
		JSON:   req,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
