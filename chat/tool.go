package chat

import (
	"encoding/json"
	"errors"
)

// ToolType is the tool discriminator.
type ToolType string

const (
	ToolFunction  ToolType = "function"
	ToolWebSearch ToolType = "deepseek_web_search"
	ToolReadURL   ToolType = "deepseek_read_url"
)

// Tool is a client function tool or a DeepSeek server-side web tool.
type Tool struct {
	Type     ToolType            `json:"type"`
	Function *FunctionDefinition `json:"function,omitempty"`
}

// FunctionDefinition describes a client function.
type FunctionDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
	Strict      *bool  `json:"strict,omitempty"`
}

// FunctionTool builds a function tool. Parameters is a JSON Schema object: a
// map, a struct or json.RawMessage.
func FunctionTool(name, description string, parameters any) Tool {
	return Tool{Type: ToolFunction, Function: &FunctionDefinition{Name: name, Description: description, Parameters: parameters}}
}

// WebSearchTool enables DeepSeek server-side web search.
func WebSearchTool() Tool { return Tool{Type: ToolWebSearch} }

// ReadURLTool enables server-side reading of links in the user message.
func ReadURLTool() Tool { return Tool{Type: ToolReadURL} }

// ToolChoiceMode is a tool_choice string value.
type ToolChoiceMode string

const (
	ToolChoiceNone     ToolChoiceMode = "none"
	ToolChoiceAuto     ToolChoiceMode = "auto"
	ToolChoiceRequired ToolChoiceMode = "required"
)

// ToolChoice is a mode string or a named function choice. Null decodes from
// JSON null and marshals back as null; it is treated as absent by Validate.
type ToolChoice struct {
	Mode     ToolChoiceMode
	Function string
	Null     bool
}

// AutoToolChoice builds tool_choice "auto".
func AutoToolChoice() *ToolChoice { return &ToolChoice{Mode: ToolChoiceAuto} }

// NoneToolChoice builds tool_choice "none".
func NoneToolChoice() *ToolChoice { return &ToolChoice{Mode: ToolChoiceNone} }

// RequiredToolChoice builds tool_choice "required"; rejected in thinking mode.
func RequiredToolChoice() *ToolChoice { return &ToolChoice{Mode: ToolChoiceRequired} }

// FunctionToolChoice forces one named function; rejected in thinking mode.
func FunctionToolChoice(name string) *ToolChoice { return &ToolChoice{Function: name} }

// MarshalJSON implements the string|object|null union.
func (t ToolChoice) MarshalJSON() ([]byte, error) {
	if t.Null {
		return []byte("null"), nil
	}
	if t.Function != "" {
		return json.Marshal(struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}{Type: "function", Function: struct {
			Name string `json:"name"`
		}{Name: t.Function}})
	}
	if t.Mode == "" {
		return nil, errors.New("deepseek/chat: tool choice has neither mode nor function")
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
		t.Mode = ToolChoiceMode(mode)
		t.Function = ""
		t.Null = false
		return nil
	}
	var object struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	if object.Type != "function" || object.Function.Name == "" {
		return errors.New("deepseek/chat: unsupported tool choice object")
	}
	t.Mode = ""
	t.Function = object.Function.Name
	t.Null = false
	return nil
}

// ToolCall is a complete function call.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall is a function name and its JSON arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCallDelta is a streamed fragment of a function call.
type ToolCallDelta struct {
	Index    int               `json:"index"`
	ID       string            `json:"id,omitempty"`
	Type     string            `json:"type,omitempty"`
	Function FunctionCallDelta `json:"function,omitempty"`
}

// FunctionCallDelta is a streamed function name or argument fragment.
type FunctionCallDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}
