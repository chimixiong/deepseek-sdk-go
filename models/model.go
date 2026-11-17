package models

// ListResponse is the model list envelope.
type ListResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// Model describes one model and its capabilities.
type Model struct {
	ID              string `json:"id"`
	Object          string `json:"object"`
	OwnedBy         string `json:"owned_by"`
	Name            string `json:"name,omitempty"`
	ContextWindow   *int   `json:"context_window,omitempty"`
	MaxOutputTokens *int   `json:"max_output_tokens,omitempty"`
	// InputModalities lists accepted input kinds: "text" or "image".
	InputModalities []string `json:"input_modalities,omitempty"`
	// OutputModalities lists generated media kinds, currently "text".
	OutputModalities []string `json:"output_modalities,omitempty"`
	// Effort lists the reasoning effort levels for thinking mode.
	Effort          *Effort          `json:"effort,omitempty"`
	APICapabilities *APICapabilities `json:"api_capabilities,omitempty"`
}

// Effort describes reasoning effort support.
type Effort struct {
	// SupportedLevels is ordered as the API recommends presenting it and
	// excludes "none", which disables thinking.
	SupportedLevels []string `json:"supported_levels"`
	// DefaultLevel is the server default when a request omits reasoning_effort.
	DefaultLevel *string `json:"default_level,omitempty"`
}

// APICapabilities lists behavior per API protocol.
type APICapabilities struct {
	AnthropicMessages *AnthropicMessages `json:"anthropic_messages,omitempty"`
}

// AnthropicMessages describes behavior through the Messages API.
type AnthropicMessages struct {
	SystemPromptUpdate SystemPromptUpdate `json:"system_prompt_update"`
}

// SystemPromptUpdate reports how a model adopts updated system prompts.
type SystemPromptUpdate string

// Documented system prompt update policies.
const (
	// SystemPromptUpdateLeadingOnly means only the leading system prompt
	// takes effect.
	SystemPromptUpdateLeadingOnly SystemPromptUpdate = "leading-only"
	// SystemPromptUpdateInHistory means a system message appended to the
	// history replaces all earlier system prompts.
	SystemPromptUpdateInHistory SystemPromptUpdate = "in-history"
)
