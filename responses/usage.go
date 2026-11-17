package responses

// Usage is the token and server-tool usage of a response.
type Usage struct {
	InputTokens         int                  `json:"input_tokens"`
	InputTokensDetails  *InputTokensDetails  `json:"input_tokens_details,omitempty"`
	OutputTokens        int                  `json:"output_tokens"`
	OutputTokensDetails *OutputTokensDetails `json:"output_tokens_details,omitempty"`
	TotalTokens         int                  `json:"total_tokens"`
	ServerToolUse       *ServerToolUse       `json:"server_tool_use,omitempty"`
}

// InputTokensDetails breaks down input tokens.
type InputTokensDetails struct {
	// CachedTokens is the number of input tokens served from the context cache.
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// OutputTokensDetails breaks down output tokens.
type OutputTokensDetails struct {
	// ReasoningTokens is the number of generated reasoning tokens.
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	// SafetyTokens is DeepSeek moderation usage, included in output_tokens and
	// returned only when safety_check is high.
	SafetyTokens *int `json:"safety_tokens,omitempty"`
}

// ServerToolUse counts successful server-side web operations. Counts are
// cumulative, not incremental. The field is present only when at least one
// count is non-zero, in which case all counts are returned.
type ServerToolUse struct {
	// ToolSearch is the number of successful web search queries.
	ToolSearch int `json:"tool_search"`
	// ToolFetch is the number of successfully fetched pages.
	ToolFetch int `json:"tool_fetch"`
	// ToolImageSearch is the number of successful image searches.
	ToolImageSearch *int `json:"tool_image_search,omitempty"`
}
