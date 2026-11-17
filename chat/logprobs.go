package chat

// ChoiceLogprobs holds log probabilities for content and reasoning tokens.
// DeepSeek returns the two lists separately.
type ChoiceLogprobs struct {
	Content          []TokenLogprob `json:"content"`
	ReasoningContent []TokenLogprob `json:"reasoning_content,omitempty"`
}

// TokenLogprob is the probability of one generated token.
type TokenLogprob struct {
	Token       string       `json:"token"`
	Logprob     float64      `json:"logprob"`
	Bytes       []int        `json:"bytes,omitempty"`
	TopLogprobs []TopLogprob `json:"top_logprobs"`
}

// TopLogprob is one alternative token in the top-N list.
type TopLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}
