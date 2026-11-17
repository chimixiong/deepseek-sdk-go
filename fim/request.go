package fim

import (
	"encoding/json"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Request is a FIM completion request.
type Request struct {
	Model    string  `json:"model"`
	Prompt   string  `json:"prompt"`
	Suffix   *string `json:"suffix,omitempty"`
	Echo     *bool   `json:"echo,omitempty"`
	Logprobs *int    `json:"logprobs,omitempty"`
	// MaxTokens is the completion token limit.
	MaxTokens *int `json:"max_tokens,omitempty"`
	// Stop is one string or up to 16 strings.
	Stop          Stop           `json:"stop,omitempty"`
	Stream        *bool          `json:"stream,omitempty"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
	Temperature   *float64       `json:"temperature,omitempty"`
	TopP          *float64       `json:"top_p,omitempty"`
	// FrequencyPenalty is deprecated by the API and has no effect.
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	// PresencePenalty is deprecated by the API and has no effect.
	PresencePenalty *float64 `json:"presence_penalty,omitempty"`
	// Extra carries fields not modeled by this package.
	Extra map[string]json.RawMessage `json:"-"`
}

// StreamOptions configures streaming.
type StreamOptions struct {
	// IncludeUsage makes every chunk carry a usage field, null except on the
	// last content chunk.
	IncludeUsage *bool `json:"include_usage,omitempty"`
}

// Stop is a stop sequence list: one item is encoded as a string, several as
// an array.
type Stop []string

// MarshalJSON encodes a single stop as a string and multiple as an array.
func (s Stop) MarshalJSON() ([]byte, error) {
	switch len(s) {
	case 0:
		return []byte("null"), nil
	case 1:
		return json.Marshal(s[0])
	default:
		return json.Marshal([]string(s))
	}
}

// UnmarshalJSON accepts null, a string or an array of strings.
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

// MarshalJSON adds Extra fields.
func (r Request) MarshalJSON() ([]byte, error) {
	type plain Request
	return deepseek.MarshalWithExtra(plain(r), r.Extra)
}

// Validate checks the documented request constraints.
func (r *Request) Validate() error {
	if r == nil {
		return deepseek.Invalid("request", "must not be nil")
	}
	if r.Model == "" {
		return deepseek.Invalid("model", "is required")
	}
	if r.Echo != nil && *r.Echo {
		if r.Suffix != nil {
			return deepseek.Invalid("echo", "cannot be combined with suffix")
		}
		if r.Logprobs != nil {
			return deepseek.Invalid("echo", "cannot be combined with logprobs")
		}
	}
	if r.Logprobs != nil && (*r.Logprobs < 0 || *r.Logprobs > 20) {
		return deepseek.Invalid("logprobs", "must be between 0 and 20")
	}
	if r.MaxTokens != nil && *r.MaxTokens < 0 {
		return deepseek.Invalid("max_tokens", "must not be negative")
	}
	if len(r.Stop) > 16 {
		return deepseek.Invalid("stop", "must contain at most 16 sequences")
	}
	if r.StreamOptions != nil && (r.Stream == nil || !*r.Stream) {
		return deepseek.Invalid("stream_options", "requires stream to be true")
	}
	if r.Temperature != nil && (*r.Temperature < 0 || *r.Temperature > 2) {
		return deepseek.Invalid("temperature", "must be between 0 and 2")
	}
	if r.TopP != nil && (*r.TopP <= 0 || *r.TopP > 1) {
		return deepseek.Invalid("top_p", "must be greater than 0 and at most 1")
	}
	return nil
}
