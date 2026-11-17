package chat

// Usage is token accounting for one request. DeepSeek reports both the generic
// prompt_tokens_details.cached_tokens and the prompt_cache_hit_tokens alias.
type Usage struct {
	CompletionTokens      int                 `json:"completion_tokens"`
	PromptTokens          int                 `json:"prompt_tokens"`
	TotalTokens           int                 `json:"total_tokens"`
	PromptTokensDetails   PromptTokensDetails `json:"prompt_tokens_details"`
	PromptCacheHitTokens  int                 `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int                 `json:"prompt_cache_miss_tokens"`
	CompletionDetails     *CompletionDetails  `json:"completion_tokens_details,omitempty"`
	ServerToolUse         *ServerToolUse      `json:"server_tool_use,omitempty"`
}

// PromptTokensDetails breaks down the prompt.
type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// CompletionDetails breaks down the completion.
type CompletionDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	SafetyTokens    int `json:"safety_tokens,omitempty"`
}

// ServerToolUse counts billed web-tool operations.
type ServerToolUse struct {
	ToolSearch      int `json:"tool_search,omitempty"`
	ToolFetch       int `json:"tool_fetch,omitempty"`
	ToolImageSearch int `json:"tool_image_search,omitempty"`
}
