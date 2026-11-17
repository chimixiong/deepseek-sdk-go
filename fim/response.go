package fim

// FinishReason reports why generation stopped.
type FinishReason string

// Documented finish reasons.
const (
	FinishStop                       FinishReason = "stop"
	FinishLength                     FinishReason = "length"
	FinishContentFilter              FinishReason = "content_filter"
	FinishInsufficientSystemResource FinishReason = "insufficient_system_resource"
	FinishAborted                    FinishReason = "aborted"
)

// Response is a text completion response. Streaming and non-streaming
// responses share this structure.
type Response struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"`
	Choices           []Choice `json:"choices"`
	Usage             *Usage   `json:"usage,omitempty"`
}

// Choice is one completion candidate.
type Choice struct {
	Text         string        `json:"text"`
	Index        int           `json:"index"`
	Logprobs     *Logprobs     `json:"logprobs"`
	FinishReason *FinishReason `json:"finish_reason"`
}

// Logprobs holds per-token probabilities.
type Logprobs struct {
	TextOffset    []int                `json:"text_offset,omitempty"`
	TokenLogprobs []float64            `json:"token_logprobs,omitempty"`
	Tokens        []string             `json:"tokens,omitempty"`
	TopLogprobs   []map[string]float64 `json:"top_logprobs,omitempty"`
}

// Usage reports token counts for the request.
type Usage struct {
	PromptTokens            int                      `json:"prompt_tokens"`
	CompletionTokens        int                      `json:"completion_tokens"`
	TotalTokens             int                      `json:"total_tokens"`
	PromptTokensDetails     *PromptTokensDetails     `json:"prompt_tokens_details,omitempty"`
	PromptCacheHitTokens    int                      `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens   int                      `json:"prompt_cache_miss_tokens"`
	CompletionTokensDetails *CompletionTokensDetails `json:"completion_tokens_details,omitempty"`
	ServerToolUse           *ServerToolUse           `json:"server_tool_use,omitempty"`
}

// PromptTokensDetails breaks down prompt tokens.
type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// CompletionTokensDetails breaks down completion tokens.
type CompletionTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	SafetyTokens    int `json:"safety_tokens,omitempty"`
}

// ServerToolUse reports billable web tool calls.
type ServerToolUse struct {
	ToolSearch      int `json:"tool_search,omitempty"`
	ToolFetch       int `json:"tool_fetch,omitempty"`
	ToolImageSearch int `json:"tool_image_search,omitempty"`
}
