package responses

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Status is a response status.
type Status string

const (
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusIncomplete Status = "incomplete"
	StatusFailed     Status = "failed"
)

// IncompleteReason explains why a response was truncated.
type IncompleteReason string

const (
	IncompleteMaxOutputTokens IncompleteReason = "max_output_tokens"
	IncompleteContentFilter   IncompleteReason = "content_filter"
)

// IncompleteDetails carries the truncation reason.
type IncompleteDetails struct {
	Reason IncompleteReason `json:"reason,omitempty"`
}

// ResponseError describes a failed response. It is an alias of
// deepseek.ResponseError so a single errors.As target works across protocols.
type ResponseError = deepseek.ResponseError

// failureError reports a failed generation carried by an otherwise successful
// HTTP response.
func (r *Response) failureError() error {
	if r == nil {
		return nil
	}
	if r.Error != nil {
		return r.Error
	}
	if r.Status == StatusFailed {
		return errors.New("deepseek/responses: response status is failed")
	}
	return nil
}

// Response is a Responses API response object. The service is stateless and
// does not store responses: Store is always false.
type Response struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	CreatedAt int64  `json:"created_at"`
	Status    Status `json:"status"`

	Background        *bool              `json:"background,omitempty"`
	CompletedAt       *int64             `json:"completed_at,omitempty"`
	ContentFilters    json.RawMessage    `json:"content_filters,omitempty"`
	Error             *ResponseError     `json:"error,omitempty"`
	FrequencyPenalty  *float64           `json:"frequency_penalty,omitempty"`
	IncompleteDetails *IncompleteDetails `json:"incomplete_details,omitempty"`
	Instructions      *string            `json:"instructions,omitempty"`
	MaxOutputTokens   *int               `json:"max_output_tokens,omitempty"`
	MaxToolCalls      *int               `json:"max_tool_calls,omitempty"`
	Model             string             `json:"model"`
	Moderation        json.RawMessage    `json:"moderation,omitempty"`
	Output            []OutputItem       `json:"output"`

	ParallelToolCalls    *bool                      `json:"parallel_tool_calls,omitempty"`
	PresencePenalty      *float64                   `json:"presence_penalty,omitempty"`
	PreviousResponseID   *string                    `json:"previous_response_id,omitempty"`
	PromptCacheKey       *string                    `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention *string                    `json:"prompt_cache_retention,omitempty"`
	Reasoning            *Reasoning                 `json:"reasoning,omitempty"`
	SafetyIdentifier     *string                    `json:"safety_identifier,omitempty"`
	ServiceTier          *string                    `json:"service_tier,omitempty"`
	Store                *bool                      `json:"store,omitempty"`
	Temperature          *float64                   `json:"temperature,omitempty"`
	Text                 *Text                      `json:"text,omitempty"`
	ToolChoice           *ToolChoice                `json:"tool_choice,omitempty"`
	Tools                []Tool                     `json:"tools,omitempty"`
	TopLogprobs          *int                       `json:"top_logprobs,omitempty"`
	TopP                 *float64                   `json:"top_p,omitempty"`
	Truncation           *string                    `json:"truncation,omitempty"`
	Usage                *Usage                     `json:"usage,omitempty"`
	User                 *string                    `json:"user,omitempty"`
	Metadata             map[string]json.RawMessage `json:"metadata,omitempty"`

	// Title is the generated conversation title, present only when the request
	// enabled it and a title was produced.
	Title string `json:"title,omitempty"`

	// Extra holds response fields this release does not model.
	Extra map[string]json.RawMessage `json:"-"`
}

type responseAlias Response

// MarshalJSON injects Extra.
func (r Response) MarshalJSON() ([]byte, error) {
	return deepseek.MarshalWithExtra(responseAlias(r), r.Extra)
}

// UnmarshalJSON keeps fields this release does not model.
func (r *Response) UnmarshalJSON(data []byte) error {
	var wire responseAlias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*r = Response(wire)
	extra, err := splitExtra(data, Response{})
	if err != nil {
		return err
	}
	r.Extra = extra
	return nil
}

// OutputText concatenates the output_text parts of all message items.
func (r *Response) OutputText() string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	for i := range r.Output {
		item := &r.Output[i]
		if item.Type != ItemMessage {
			continue
		}
		for j := range item.Content {
			part := &item.Content[j]
			if part.Type == PartOutputText {
				b.WriteString(part.Text)
			}
		}
	}
	return b.String()
}
