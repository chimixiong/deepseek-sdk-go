package chat

import (
	"encoding/json"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Request is a Chat Completions request.
type Request struct {
	Model           string          `json:"model"`
	Messages        []Message       `json:"messages"`
	Thinking        *Thinking       `json:"thinking,omitempty"`
	ReasoningEffort ReasoningEffort `json:"reasoning_effort,omitempty"`
	MaxTokens       *int            `json:"max_tokens,omitempty"`
	ResponseFormat  *ResponseFormat `json:"response_format,omitempty"`
	Stop            Stop            `json:"stop,omitempty"`
	Stream          *bool           `json:"stream,omitempty"`
	StreamOptions   *StreamOptions  `json:"stream_options,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	TopP            *float64        `json:"top_p,omitempty"`
	Tools           []Tool          `json:"tools,omitempty"`
	ToolChoice      *ToolChoice     `json:"tool_choice,omitempty"`
	SafetyCheck     *SafetyCheck    `json:"safety_check,omitempty"`
	Title           *Title          `json:"title,omitempty"`
	Logprobs        *bool           `json:"logprobs,omitempty"`
	TopLogprobs     *int            `json:"top_logprobs,omitempty"`
	UserID          string          `json:"user_id,omitempty"`
	// FrequencyPenalty and PresencePenalty are deprecated by DeepSeek: the
	// fields are accepted for compatibility but have no effect.
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
	// Extra merges additional top-level fields into the request body.
	Extra map[string]json.RawMessage `json:"-"`
}

type requestAlias Request

// MarshalJSON injects Extra.
func (r Request) MarshalJSON() ([]byte, error) {
	return deepseek.MarshalWithExtra(requestAlias(r), r.Extra)
}

// StreamOptions controls streaming output.
type StreamOptions struct {
	IncludeUsage *bool `json:"include_usage,omitempty"`
}

// Thinking controls whether thinking mode is used.
type Thinking struct {
	Type ThinkingType `json:"type"`
}

// ThinkingType is the thinking mode switch.
type ThinkingType string

const (
	ThinkingEnabled  ThinkingType = "enabled"
	ThinkingDisabled ThinkingType = "disabled"
)

// ReasoningEffort is the thinking intensity. The service maps "minimal" to
// "low" and "medium"/"xhigh" to "high".
type ReasoningEffort string

const (
	EffortNone ReasoningEffort = "none"
	EffortLow  ReasoningEffort = "low"
	EffortHigh ReasoningEffort = "high"
	EffortMax  ReasoningEffort = "max"

	// EffortMinimal, EffortMedium and EffortXHigh are documented aliases.
	EffortMinimal ReasoningEffort = "minimal"
	EffortMedium  ReasoningEffort = "medium"
	EffortXHigh   ReasoningEffort = "xhigh"
)

// ResponseFormat selects the output format.
type ResponseFormat struct {
	Type ResponseFormatType `json:"type"`
}

// ResponseFormatType is text or JSON object output.
type ResponseFormatType string

const (
	FormatText       ResponseFormatType = "text"
	FormatJSONObject ResponseFormatType = "json_object"
)

// SafetyCheck configures DeepSeek content moderation.
type SafetyCheck struct {
	Level SafetyLevel `json:"level"`
}

// SafetyLevel is the moderation level.
type SafetyLevel string

const (
	SafetyLow  SafetyLevel = "low"
	SafetyHigh SafetyLevel = "high"
)

// Title requests a generated conversation title.
type Title struct {
	Enabled bool `json:"enabled"`
}

// Stop is a string or up to 16 strings; it marshals one value as a string and
// several as an array, matching the API union.
type Stop []string

// MarshalJSON implements the string|string[] union.
func (s Stop) MarshalJSON() ([]byte, error) {
	if len(s) == 1 {
		return json.Marshal(s[0])
	}
	return json.Marshal([]string(s))
}

// UnmarshalJSON accepts a string, an array or null. Null decodes to a nil
// slice so a decoded request omits stop on re-marshal.
func (s *Stop) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = nil
		return nil
	}
	var one string
	if err := json.Unmarshal(data, &one); err == nil {
		*s = Stop{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return err
	}
	*s = many
	return nil
}
