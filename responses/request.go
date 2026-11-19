package responses

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Request is a Responses API request. Either Input or Instructions must be
// set; Input and Instructions together are the whole conversation because the
// API is stateless.
type Request struct {
	Model string `json:"model"`
	Input *Input `json:"input,omitempty"`
	// Instructions become the first system message.
	Instructions *string    `json:"instructions,omitempty"`
	Reasoning    *Reasoning `json:"reasoning,omitempty"`
	// MaxOutputTokens caps visible output plus reasoning tokens.
	MaxOutputTokens *int         `json:"max_output_tokens,omitempty"`
	Stream          *bool        `json:"stream,omitempty"`
	Temperature     *float64     `json:"temperature,omitempty"`
	TopP            *float64     `json:"top_p,omitempty"`
	Text            *Text        `json:"text,omitempty"`
	Tools           []Tool       `json:"tools,omitempty"`
	ToolChoice      *ToolChoice  `json:"tool_choice,omitempty"`
	TopLogprobs     *int         `json:"top_logprobs,omitempty"`
	User            *string      `json:"user,omitempty"`
	SafetyCheck     *SafetyCheck `json:"safety_check,omitempty"`
	Title           *Title       `json:"title,omitempty"`
	// Extra merges additional top-level fields into the request body.
	Extra map[string]json.RawMessage `json:"-"`
}

type requestAlias Request

// MarshalJSON injects Extra.
func (r Request) MarshalJSON() ([]byte, error) {
	return deepseek.MarshalWithExtra(requestAlias(r), r.Extra)
}

func (r *Request) prepared(stream bool) *Request {
	wire := *r
	wire.Stream = &stream
	return &wire
}

func (r *Request) hasInput() bool {
	if r.Input == nil || r.Input.Null {
		return false
	}
	return r.Input.Items != nil || r.Input.Text != ""
}

// Validate reports violations of documented request constraints. It does not
// call the API.
func (r *Request) Validate() error {
	if r == nil {
		return deepseek.Invalid("request", "is nil")
	}
	if r.Model == "" {
		return deepseek.Invalid("model", "is required")
	}
	if !r.hasInput() && (r.Instructions == nil || *r.Instructions == "") {
		return deepseek.Invalid("input", "input or instructions is required")
	}
	if r.MaxOutputTokens != nil && *r.MaxOutputTokens < 1 {
		return deepseek.Invalid("max_output_tokens", "must be at least 1, got %d", *r.MaxOutputTokens)
	}
	if r.Temperature != nil && (*r.Temperature < 0 || *r.Temperature > 2) {
		return deepseek.Invalid("temperature", "must be between 0 and 2, got %v", *r.Temperature)
	}
	if r.TopP != nil && (*r.TopP < 0 || *r.TopP > 1) {
		return deepseek.Invalid("top_p", "must be between 0 and 1")
	}
	if r.TopLogprobs != nil && (*r.TopLogprobs < 0 || *r.TopLogprobs > 20) {
		return deepseek.Invalid("top_logprobs", "must be between 0 and 20, got %d", *r.TopLogprobs)
	}
	if r.User != nil && len(*r.User) > 512 {
		return deepseek.Invalid("user", "must not exceed 512 bytes, got %d", len(*r.User))
	}
	if r.SafetyCheck != nil {
		switch r.SafetyCheck.Level {
		case SafetyLow, SafetyHigh:
		default:
			return deepseek.Invalid("safety_check.level", "must be %q or %q, got %q", SafetyLow, SafetyHigh, r.SafetyCheck.Level)
		}
	}
	if r.Reasoning != nil && r.Reasoning.Effort != "" {
		switch r.Reasoning.Effort {
		case EffortNone, EffortLow, EffortHigh, EffortMax, EffortMinimal, EffortMedium, EffortXHigh:
		default:
			return deepseek.Invalid("reasoning.effort", "unsupported value %q", r.Reasoning.Effort)
		}
	}
	if err := r.Text.validate(); err != nil {
		return err
	}
	if err := validateToolChoice(r.ToolChoice); err != nil {
		return err
	}
	if err := validateTools(r.Tools); err != nil {
		return err
	}
	return validateInput(r.Input)
}

// Input is the request input: a plain string (one user message), a list of
// items, or JSON null. The zero value marshals as an empty string; use Null to
// send null.
type Input struct {
	Text  string
	Items []InputItem
	Null  bool
}

// StringInput builds a string input.
func StringInput(text string) *Input { return &Input{Text: text} }

// ItemsInput builds an item-list input.
func ItemsInput(items ...InputItem) *Input {
	if items == nil {
		items = []InputItem{}
	}
	return &Input{Items: items}
}

// NullInput builds a JSON null input.
func NullInput() *Input { return &Input{Null: true} }

// MarshalJSON implements the string|item[]|null union.
func (in Input) MarshalJSON() ([]byte, error) {
	if in.Null {
		return []byte("null"), nil
	}
	if in.Items != nil {
		return json.Marshal(in.Items)
	}
	return json.Marshal(in.Text)
}

// UnmarshalJSON accepts a string, an item array or null.
func (in *Input) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*in = Input{Null: true}
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*in = Input{Text: text}
		return nil
	}
	var items []InputItem
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	*in = Input{Items: items}
	return nil
}

// ReasoningEffort controls thinking mode and intensity. "none" disables
// thinking; "low", "high" and "max" enable it. For compatibility the service
// maps "minimal" to "low" and "medium"/"xhigh" to "high".
type ReasoningEffort string

const (
	EffortNone ReasoningEffort = "none"
	EffortLow  ReasoningEffort = "low"
	EffortHigh ReasoningEffort = "high"
	EffortMax  ReasoningEffort = "max"

	// EffortMinimal, EffortMedium and EffortXHigh are compatibility aliases
	// accepted by the service.
	EffortMinimal ReasoningEffort = "minimal"
	EffortMedium  ReasoningEffort = "medium"
	EffortXHigh   ReasoningEffort = "xhigh"
)

// Reasoning configures thinking mode.
type Reasoning struct {
	Effort ReasoningEffort `json:"effort,omitempty"`
	// Summary is accepted but no summary is generated.
	Summary *string `json:"summary,omitempty"`
}

// Text is the output text configuration.
type Text struct {
	Format *TextFormat `json:"format,omitempty"`
	// Verbosity is accepted but has no effect.
	Verbosity *string `json:"verbosity,omitempty"`
}

// TextFormatType selects plain text, JSON mode or a JSON schema.
type TextFormatType string

const (
	FormatText       TextFormatType = "text"
	FormatJSONObject TextFormatType = "json_object"
	FormatJSONSchema TextFormatType = "json_schema"
)

// TextFormat is the output format. Name and Schema are required when Type is
// json_schema.
type TextFormat struct {
	Type   TextFormatType `json:"type"`
	Name   string         `json:"name,omitempty"`
	Schema map[string]any `json:"schema,omitempty"`
}

func (t *Text) validate() error {
	if t == nil || t.Format == nil {
		return nil
	}
	f := t.Format
	switch f.Type {
	case FormatText, FormatJSONObject, FormatJSONSchema:
	case "":
		return deepseek.Invalid("text.format.type", "is required")
	default:
		return deepseek.Invalid("text.format.type", "unsupported value %q", f.Type)
	}
	if f.Type != FormatJSONSchema {
		return nil
	}
	if f.Name == "" {
		return deepseek.Invalid("text.format.name", "is required for json_schema")
	}
	if f.Schema == nil {
		return deepseek.Invalid("text.format.schema", "is required for json_schema")
	}
	return nil
}

// ToolType is the tool discriminator.
type ToolType string

const (
	ToolFunction          ToolType = "function"
	ToolCustom            ToolType = "custom"
	ToolWebSearch         ToolType = "web_search"
	ToolDeepSeekWebSearch ToolType = "deepseek_web_search"
	ToolDeepSeekReadURL   ToolType = "deepseek_read_url"
)

// Tool is a tool the model may call. Function fields live at the top level in
// the Responses format. Unknown types are passed through to the service.
type Tool struct {
	Type        ToolType `json:"type"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Parameters  any      `json:"parameters,omitempty"`
	// Extra merges additional top-level fields into the tool object.
	Extra map[string]json.RawMessage `json:"-"`
}

type toolAlias Tool

// MarshalJSON injects Extra.
func (t Tool) MarshalJSON() ([]byte, error) {
	return deepseek.MarshalWithExtra(toolAlias(t), t.Extra)
}

// UnmarshalJSON keeps fields this release does not model.
func (t *Tool) UnmarshalJSON(data []byte) error {
	var wire toolAlias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*t = Tool(wire)
	extra, err := splitExtra(data, Tool{})
	if err != nil {
		return err
	}
	t.Extra = extra
	return nil
}

// FunctionTool builds a function tool. Parameters is a JSON Schema object: a
// map, a struct or json.RawMessage.
func FunctionTool(name, description string, parameters any) Tool {
	return Tool{Type: ToolFunction, Name: name, Description: description, Parameters: parameters}
}

// CustomTool builds a custom tool. The service only accepts "apply_patch".
func CustomTool(name string) Tool { return Tool{Type: ToolCustom, Name: name} }

// WebSearchTool enables server-side web search, supported by deepseek-v4-pro.
func WebSearchTool() Tool { return Tool{Type: ToolWebSearch} }

// DeepSeekWebSearchTool enables the DeepSeek web search tool.
func DeepSeekWebSearchTool() Tool { return Tool{Type: ToolDeepSeekWebSearch} }

// DeepSeekReadURLTool enables reading links from the user message.
func DeepSeekReadURLTool() Tool { return Tool{Type: ToolDeepSeekReadURL} }

// ToolChoiceMode is a tool_choice string value.
type ToolChoiceMode string

const (
	ToolChoiceNone     ToolChoiceMode = "none"
	ToolChoiceAuto     ToolChoiceMode = "auto"
	ToolChoiceRequired ToolChoiceMode = "required"
)

// ToolChoice is a mode string or a named function. Set exactly one of Mode and
// Function; the named form marshals as {"type":"function","name":...}. Null
// marshals as JSON null.
type ToolChoice struct {
	Mode     ToolChoiceMode
	Function string
	Null     bool
}

// AutoToolChoice builds tool_choice "auto".
func AutoToolChoice() *ToolChoice { return &ToolChoice{Mode: ToolChoiceAuto} }

// NoneToolChoice builds tool_choice "none".
func NoneToolChoice() *ToolChoice { return &ToolChoice{Mode: ToolChoiceNone} }

// RequiredToolChoice builds tool_choice "required".
func RequiredToolChoice() *ToolChoice { return &ToolChoice{Mode: ToolChoiceRequired} }

// FunctionToolChoice forces one named function.
func FunctionToolChoice(name string) *ToolChoice { return &ToolChoice{Function: name} }

// MarshalJSON implements the string|object|null union.
func (t ToolChoice) MarshalJSON() ([]byte, error) {
	if t.Null {
		return []byte("null"), nil
	}
	if t.Function != "" {
		return json.Marshal(struct {
			Type string `json:"type"`
			Name string `json:"name"`
		}{Type: "function", Name: t.Function})
	}
	if t.Mode == "" {
		return nil, errors.New("deepseek/responses: tool choice has neither mode nor function")
	}
	return json.Marshal(t.Mode)
}

// UnmarshalJSON accepts a mode string, a named function object or null.
func (t *ToolChoice) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*t = ToolChoice{Null: true}
		return nil
	}
	var mode string
	if err := json.Unmarshal(data, &mode); err == nil {
		*t = ToolChoice{Mode: ToolChoiceMode(mode)}
		return nil
	}
	var object struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	if object.Type != "function" || object.Name == "" {
		return errors.New("deepseek/responses: unsupported tool choice object")
	}
	*t = ToolChoice{Function: object.Name}
	return nil
}

func validateToolChoice(tc *ToolChoice) error {
	if tc == nil || tc.Null {
		return nil
	}
	switch {
	case tc.Mode != "" && tc.Function != "":
		return deepseek.Invalid("tool_choice", "must be a mode or a named function, not both")
	case tc.Mode == "" && tc.Function == "":
		return deepseek.Invalid("tool_choice", `must be "none", "auto", "required" or a named function`)
	case tc.Mode != "":
		switch tc.Mode {
		case ToolChoiceNone, ToolChoiceAuto, ToolChoiceRequired:
		default:
			return deepseek.Invalid("tool_choice", "unsupported mode %q", tc.Mode)
		}
	}
	return nil
}

var functionNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func validateTools(tools []Tool) error {
	names := make(map[string]bool, len(tools))
	for i, tool := range tools {
		field := fmt.Sprintf("tools[%d]", i)
		if tool.Type == "" {
			return deepseek.Invalid(field+".type", "is required")
		}
		switch tool.Type {
		case ToolFunction:
			if tool.Name == "" {
				return deepseek.Invalid(field+".name", "is required")
			}
			if len(tool.Name) > 128 {
				return deepseek.Invalid(field+".name", "must be at most 128 characters, got %d", len(tool.Name))
			}
			if !functionNamePattern.MatchString(tool.Name) {
				return deepseek.Invalid(field+".name", "must match ^[a-zA-Z0-9_-]+$, got %q", tool.Name)
			}
		case ToolCustom:
			if tool.Name != "apply_patch" {
				return deepseek.Invalid(field+".name", `must be "apply_patch", got %q`, tool.Name)
			}
		}
		if tool.Name == "" {
			continue
		}
		if names[tool.Name] {
			return deepseek.Invalid(field+".name", "duplicate tool name %q", tool.Name)
		}
		names[tool.Name] = true
	}
	return nil
}

func validateInput(in *Input) error {
	if in == nil || in.Null {
		return nil
	}
	type callRef struct {
		field  string
		kind   ItemType
		output ItemType
	}
	calls := make(map[string]callRef, len(in.Items))
	outputs := make(map[string]ItemType, len(in.Items))
	var callOrder []string
	for i, item := range in.Items {
		field := fmt.Sprintf("input[%d]", i)
		switch item.Type {
		case ItemFunctionCall, ItemFunctionCallOutput, ItemCustomToolCall, ItemCustomToolCallOutput:
			if item.CallID == "" {
				return deepseek.Invalid(field+".call_id", "is required for %s items", item.Type)
			}
		}
		switch item.Type {
		case ItemFunctionCall, ItemCustomToolCall:
			output := ItemFunctionCallOutput
			if item.Type == ItemCustomToolCall {
				output = ItemCustomToolCallOutput
			}
			if _, exists := calls[item.CallID]; exists {
				return deepseek.Invalid(field+".call_id", "duplicate call_id %q", item.CallID)
			}
			calls[item.CallID] = callRef{field: field, kind: item.Type, output: output}
			callOrder = append(callOrder, item.CallID)
		case ItemFunctionCallOutput, ItemCustomToolCallOutput:
			if _, exists := outputs[item.CallID]; exists {
				return deepseek.Invalid(field+".call_id", "duplicate call_id %q", item.CallID)
			}
			outputs[item.CallID] = item.Type
		}
		if item.Type == ItemCustomToolCall && item.Name != "apply_patch" {
			return deepseek.Invalid(field+".name", `custom tool calls must name "apply_patch", got %q`, item.Name)
		}
		if item.Content != nil {
			if err := validateContentParts(field+".content", item.Content.Parts); err != nil {
				return err
			}
		}
		if item.Output != nil {
			if err := validateContentParts(field+".output", item.Output.Parts); err != nil {
				return err
			}
		}
	}
	for _, id := range callOrder {
		call := calls[id]
		if outputs[id] != call.output {
			return deepseek.Invalid(call.field+".call_id", "%s %q has no matching %s item", call.kind, id, call.output)
		}
	}
	return nil
}

func validateContentParts(field string, parts []ContentPart) error {
	for i, part := range parts {
		partField := fmt.Sprintf("%s[%d]", field, i)
		switch part.Type {
		case PartInputImage:
			switch {
			case part.ImageURL == "" && part.FileID == "":
				return deepseek.Invalid(partField, "input_image must have image_url or file_id")
			case part.ImageURL != "" && part.FileID != "":
				return deepseek.Invalid(partField, "input_image cannot have both image_url and file_id")
			}
		case PartInputFile:
			if part.FileID == "" {
				return deepseek.Invalid(partField+".file_id", "is required")
			}
		}
	}
	return nil
}

// SafetyCheck enables DeepSeek content moderation.
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
