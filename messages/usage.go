package messages

// ServiceTier is the processing tier reported in Usage.
type ServiceTier string

// ServiceTierStandard is the only tier the API reports.
const ServiceTierStandard ServiceTier = "standard"

// Usage reports token consumption for one messages request.
type Usage struct {
	// InputTokens excludes cache hits.
	InputTokens int `json:"input_tokens"`
	// CacheCreationInputTokens is always 0; the API manages the cache itself.
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	// CacheReadInputTokens counts cache hits.
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
	// OutputTokens includes thinking tokens and is cumulative in message_delta.
	OutputTokens        int                  `json:"output_tokens"`
	OutputTokensDetails *OutputTokensDetails `json:"output_tokens_details,omitempty"`
	ServiceTier         ServiceTier          `json:"service_tier"`
	// ServerToolUse is present only when a server tool ran.
	ServerToolUse *ServerToolUsage `json:"server_tool_use,omitempty"`
}

// OutputTokensDetails breaks down output tokens.
type OutputTokensDetails struct {
	// ThinkingTokens is included in OutputTokens.
	ThinkingTokens int `json:"thinking_tokens,omitempty"`
	// SafetyTokens is a DeepSeek extension, included in OutputTokens.
	SafetyTokens int `json:"safety_tokens,omitempty"`
}

// ServerToolUsage counts server tool calls. Counts are cumulative in
// message_delta.
type ServerToolUsage struct {
	// WebSearchRequests counts attempts, including failed ones.
	WebSearchRequests int `json:"web_search_requests"`
	// ToolSearch counts successful searches returning results.
	ToolSearch int `json:"tool_search"`
	// WebFetchRequests counts fetch result blocks, including errors.
	WebFetchRequests *int `json:"web_fetch_requests,omitempty"`
	// ToolFetch counts billable page fetches.
	ToolFetch *int `json:"tool_fetch,omitempty"`
	// ToolImageSearch counts billable image searches.
	ToolImageSearch *int `json:"tool_image_search,omitempty"`
}

func cloneUsage(usage Usage) Usage {
	if usage.OutputTokensDetails != nil {
		details := *usage.OutputTokensDetails
		usage.OutputTokensDetails = &details
	}
	if usage.ServerToolUse != nil {
		usage.ServerToolUse = cloneServerToolUsage(usage.ServerToolUse)
	}
	return usage
}

func cloneServerToolUsage(tools *ServerToolUsage) *ServerToolUsage {
	if tools == nil {
		return nil
	}
	cloned := *tools
	cloned.WebFetchRequests = cloneInt(cloned.WebFetchRequests)
	cloned.ToolFetch = cloneInt(cloned.ToolFetch)
	cloned.ToolImageSearch = cloneInt(cloned.ToolImageSearch)
	return &cloned
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	return new(*value)
}

// mergeUsage folds a message_delta usage into the accumulated usage.
// Anthropic sends output_tokens as a cumulative total and only repeats the
// message_start counters when the cache changed, so every counter is merged by
// maximum: a delta that omits a counter leaves the message_start value intact,
// and a cumulative delta never shrinks the total. ServiceTier is replaced when
// the delta sets it.
func mergeUsage(dst *Usage, delta *Usage) {
	if delta == nil {
		return
	}
	dst.InputTokens = max(dst.InputTokens, delta.InputTokens)
	dst.CacheCreationInputTokens = max(dst.CacheCreationInputTokens, delta.CacheCreationInputTokens)
	dst.CacheReadInputTokens = max(dst.CacheReadInputTokens, delta.CacheReadInputTokens)
	dst.OutputTokens = max(dst.OutputTokens, delta.OutputTokens)
	if delta.OutputTokensDetails != nil {
		if dst.OutputTokensDetails == nil {
			dst.OutputTokensDetails = &OutputTokensDetails{}
		}
		dst.OutputTokensDetails.ThinkingTokens = max(dst.OutputTokensDetails.ThinkingTokens, delta.OutputTokensDetails.ThinkingTokens)
		dst.OutputTokensDetails.SafetyTokens = max(dst.OutputTokensDetails.SafetyTokens, delta.OutputTokensDetails.SafetyTokens)
	}
	if delta.ServiceTier != "" {
		dst.ServiceTier = delta.ServiceTier
	}
	if delta.ServerToolUse == nil {
		return
	}
	if dst.ServerToolUse == nil {
		dst.ServerToolUse = cloneServerToolUsage(delta.ServerToolUse)
		return
	}
	dst.ServerToolUse.WebSearchRequests = max(dst.ServerToolUse.WebSearchRequests, delta.ServerToolUse.WebSearchRequests)
	dst.ServerToolUse.ToolSearch = max(dst.ServerToolUse.ToolSearch, delta.ServerToolUse.ToolSearch)
	dst.ServerToolUse.WebFetchRequests = maxCountPtr(dst.ServerToolUse.WebFetchRequests, delta.ServerToolUse.WebFetchRequests)
	dst.ServerToolUse.ToolFetch = maxCountPtr(dst.ServerToolUse.ToolFetch, delta.ServerToolUse.ToolFetch)
	dst.ServerToolUse.ToolImageSearch = maxCountPtr(dst.ServerToolUse.ToolImageSearch, delta.ServerToolUse.ToolImageSearch)
}

func maxCountPtr(dst, delta *int) *int {
	if delta == nil {
		return dst
	}
	if dst == nil {
		return new(*delta)
	}
	return new(max(*dst, *delta))
}
