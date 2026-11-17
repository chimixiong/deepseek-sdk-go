package chat

import "encoding/json"

// FinishReason is why generation stopped.
type FinishReason string

const (
	FinishStop                  FinishReason = "stop"
	FinishLength                FinishReason = "length"
	FinishContentFilter         FinishReason = "content_filter"
	FinishToolCalls             FinishReason = "tool_calls"
	FinishInsufficientSystemRes FinishReason = "insufficient_system_resource"
	FinishAborted               FinishReason = "aborted"
)

// Response is a non-streaming chat completion.
type Response struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	SystemFingerprint string   `json:"system_fingerprint"`
	Choices           []Choice `json:"choices"`
	Usage             *Usage   `json:"usage,omitempty"`
}

// Choice is one completion candidate.
type Choice struct {
	Index        int             `json:"index"`
	Message      Message         `json:"message"`
	FinishReason FinishReason    `json:"finish_reason"`
	Logprobs     *ChoiceLogprobs `json:"logprobs,omitempty"`
}

// AssistantMessage returns the first choice's assistant message.
func (r *Response) AssistantMessage() Message {
	if r == nil || len(r.Choices) == 0 {
		return Message{}
	}
	return r.Choices[0].Message
}

// ReasoningContent returns the first choice's reasoning text, if any.
func (r *Response) ReasoningContent() string {
	message := r.AssistantMessage()
	if message.ReasoningContent == nil {
		return ""
	}
	return *message.ReasoningContent
}

// RawAnnotations returns the first choice's annotations untouched.
func (r *Response) RawAnnotations() []json.RawMessage {
	return r.AssistantMessage().Annotations
}
