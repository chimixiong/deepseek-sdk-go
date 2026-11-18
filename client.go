// Package deepseek is the shared transport for the DeepSeek HTTP APIs:
// authentication, retries, JSON, multipart and server-sent events.
//
// Protocol types live in the chat, responses, messages, fim, models, billing,
// files and tools subpackages.
package deepseek

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the DeepSeek API root.
	DefaultBaseURL = "https://api.deepseek.com"
	// Version is reported in the User-Agent header and matches the released
	// tag.
	Version = "0.0.1"

	defaultMaxRetries     = 2
	defaultRetryBaseDelay = 500 * time.Millisecond
	maxRetryDelay         = 8 * time.Second
)

// Config configures a Client. The zero value is usable: the API key is read
// from DEEPSEEK_API_KEY and the base URL defaults to DefaultBaseURL.
type Config struct {
	// APIKey is sent as "Authorization: Bearer <key>". When empty, New falls
	// back to DEEPSEEK_API_KEY; if that is empty too, no header is sent.
	APIKey string
	// BaseURL defaults to DefaultBaseURL. Trailing slashes are trimmed.
	BaseURL string
	// BetaBaseURL defaults to BaseURL + "/beta" and serves the FIM API and the
	// beta chat completions endpoint.
	BetaBaseURL string
	// AnthropicBaseURL defaults to BaseURL + "/anthropic" and serves the
	// Messages API.
	AnthropicBaseURL string
	// HTTPClient defaults to a client without a timeout: use context deadlines.
	HTTPClient *http.Client
	// MaxRetries is the number of retries after the first attempt. Zero means
	// the default (2); a negative value disables retries.
	MaxRetries int
	// RetryBaseDelay is the first backoff step, doubled per attempt and capped
	// at 8s. Zero means 500ms; a negative value keeps retries but skips the
	// wait between attempts.
	RetryBaseDelay time.Duration
	// UserAgent defaults to
	// "deepseek-sdk-go/<Version> (+https://github.com/chimixiong/deepseek-sdk-go)".
	UserAgent string
}

// Client talks to the DeepSeek HTTP APIs.
type Client struct {
	apiKey           string
	baseURL          string
	betaBaseURL      string
	anthropicBaseURL string
	httpClient       *http.Client
	maxRetries       int
	retryBaseDelay   time.Duration
	userAgent        string
}

// New returns a Client. It never fails; missing configuration is filled with
// defaults and an absent API key surfaces as an HTTP error on the first call.
func New(cfg Config) *Client {
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("DEEPSEEK_API_KEY")
	}
	baseURL := trimSlash(cfg.BaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	betaBaseURL := trimSlash(cfg.BetaBaseURL)
	if betaBaseURL == "" {
		betaBaseURL = baseURL + "/beta"
	}
	anthropicBaseURL := trimSlash(cfg.AnthropicBaseURL)
	if anthropicBaseURL == "" {
		anthropicBaseURL = baseURL + "/anthropic"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{}
	}
	maxRetries := cfg.MaxRetries
	switch {
	case maxRetries == 0:
		maxRetries = defaultMaxRetries
	case maxRetries < 0:
		maxRetries = 0
	}
	retryBaseDelay := cfg.RetryBaseDelay
	switch {
	case retryBaseDelay == 0:
		retryBaseDelay = defaultRetryBaseDelay
	case retryBaseDelay < 0:
		retryBaseDelay = 0
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "deepseek-sdk-go/" + Version + " (+https://github.com/chimixiong/deepseek-sdk-go)"
	}
	return &Client{
		apiKey:           cfg.APIKey,
		baseURL:          baseURL,
		betaBaseURL:      betaBaseURL,
		anthropicBaseURL: anthropicBaseURL,
		httpClient:       cfg.HTTPClient,
		maxRetries:       maxRetries,
		retryBaseDelay:   retryBaseDelay,
		userAgent:        cfg.UserAgent,
	}
}

// APIKey returns the configured key.
func (c *Client) APIKey() string { return c.apiKey }

// BaseURL returns the root used by most endpoints.
func (c *Client) BaseURL() string { return c.baseURL }

// BetaBaseURL returns the root used by beta endpoints (FIM, chat prefix).
func (c *Client) BetaBaseURL() string { return c.betaBaseURL }

// AnthropicBaseURL returns the root used by the Messages API.
func (c *Client) AnthropicBaseURL() string { return c.anthropicBaseURL }

// HTTPClient returns the underlying client.
func (c *Client) HTTPClient() *http.Client { return c.httpClient }

// ChatCompletionsURL is the standard Chat Completions endpoint.
func (c *Client) ChatCompletionsURL() string { return c.baseURL + "/chat/completions" }

// BetaChatCompletionsURL is the beta Chat Completions endpoint, required for
// assistant prefix continuation.
func (c *Client) BetaChatCompletionsURL() string {
	return c.betaBaseURL + "/chat/completions"
}

// ResponsesURL is the Responses endpoint.
func (c *Client) ResponsesURL() string { return c.baseURL + "/responses" }

// FIMURL is the FIM completion endpoint.
func (c *Client) FIMURL() string { return c.betaBaseURL + "/completions" }

// MessagesURL is the Anthropic-compatible Messages endpoint.
func (c *Client) MessagesURL() string { return c.anthropicBaseURL + "/v1/messages" }

// MessagesCompatURL is the short Anthropic-compatible Messages path.
func (c *Client) MessagesCompatURL() string { return c.anthropicBaseURL + "/messages" }

// ModelsURL is the model list endpoint.
func (c *Client) ModelsURL() string { return c.baseURL + "/models" }

// BalanceURL is the account balance endpoint.
func (c *Client) BalanceURL() string { return c.baseURL + "/user/balance" }

// FilesURL is the file collection endpoint.
func (c *Client) FilesURL() string { return c.baseURL + "/files" }

// FileURL is the endpoint for one file.
func (c *Client) FileURL(id string) string {
	return c.baseURL + "/files/" + url.PathEscape(id)
}

// ToolURL is the endpoint for a tool, for example "search" or "tts".
func (c *Client) ToolURL(name string) string {
	return c.baseURL + "/v1/tools/" + url.PathEscape(name)
}

func trimSlash(s string) string { return strings.TrimRight(s, "/") }
