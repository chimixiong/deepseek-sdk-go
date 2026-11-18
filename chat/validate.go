package chat

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/chimixiong/deepseek-sdk-go"
)

// toolNamePattern matches the documented function name constraint.
var toolNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Validate checks the documented structural constraints without a network
// call. Streaming-only fields are rejected here; CreateStream uses an internal
// variant that allows them.
func (r *Request) Validate() error { return r.validate(false) }

func (r *Request) validate(streaming bool) error {
	if r == nil {
		return deepseek.Invalid("request", "is nil")
	}
	if r.Model == "" {
		return deepseek.Invalid("model", "is required")
	}
	if len(r.Messages) == 0 {
		return deepseek.Invalid("messages", "must contain at least one message")
	}
	for i := range r.Messages {
		if err := validateMessage(i, r.Messages[i]); err != nil {
			return err
		}
		if r.Messages[i].Role == RoleLatestReminder {
			if i+1 >= len(r.Messages) || r.Messages[i+1].Role != RoleUser {
				return deepseek.Invalid(fmt.Sprintf("messages[%d].role", i), "latest_reminder must be immediately followed by a user message")
			}
		}
	}
	if len(r.Stop) > 16 {
		return deepseek.Invalid("stop", "contains %d entries, maximum is 16", len(r.Stop))
	}
	for _, sequence := range r.Stop {
		if sequence == "" {
			return deepseek.Invalid("stop", "must not contain an empty sequence")
		}
		if len(sequence) > 512 {
			return deepseek.Invalid("stop", "each sequence must not exceed 512 bytes, got %d", len(sequence))
		}
	}
	if r.MaxTokens != nil && (*r.MaxTokens < 1 || *r.MaxTokens > 393216) {
		return deepseek.Invalid("max_tokens", "must be between 1 and 393216")
	}
	if r.Temperature != nil && (*r.Temperature < 0 || *r.Temperature > 2) {
		return deepseek.Invalid("temperature", "must be between 0 and 2")
	}
	if r.TopP != nil && (*r.TopP <= 0 || *r.TopP > 1) {
		return deepseek.Invalid("top_p", "must be greater than 0 and at most 1")
	}
	if r.TopLogprobs != nil {
		if *r.TopLogprobs < 0 || *r.TopLogprobs > 20 {
			return deepseek.Invalid("top_logprobs", "must be between 0 and 20")
		}
		if r.Logprobs == nil || !*r.Logprobs {
			return deepseek.Invalid("top_logprobs", "requires logprobs to be true")
		}
	}
	if r.StreamOptions != nil && !streaming && (r.Stream == nil || !*r.Stream) {
		return deepseek.Invalid("stream_options", "is only valid for CreateStream")
	}
	if r.ResponseFormat != nil {
		switch r.ResponseFormat.Type {
		case FormatText, FormatJSONObject:
		default:
			return deepseek.Invalid("response_format.type", "unsupported value %q", r.ResponseFormat.Type)
		}
	}
	if r.Thinking != nil {
		switch r.Thinking.Type {
		case ThinkingEnabled, ThinkingDisabled:
		default:
			return deepseek.Invalid("thinking.type", "unsupported value %q", r.Thinking.Type)
		}
	}
	if r.ReasoningEffort != "" {
		switch r.ReasoningEffort {
		case EffortNone, EffortLow, EffortHigh, EffortMax,
			EffortMinimal, EffortMedium, EffortXHigh: // documented aliases
		default:
			return deepseek.Invalid("reasoning_effort", "unsupported value %q", r.ReasoningEffort)
		}
	}
	if r.SafetyCheck != nil {
		switch r.SafetyCheck.Level {
		case SafetyLow, SafetyHigh:
		default:
			return deepseek.Invalid("safety_check.level", "unsupported value %q", r.SafetyCheck.Level)
		}
	}
	if len(r.UserID) > 512 {
		return deepseek.Invalid("user_id", "must not exceed 512 bytes")
	}
	if len(r.Tools) > 0 {
		seen := make(map[string]bool, len(r.Tools))
		for i, tool := range r.Tools {
			field := fmt.Sprintf("tools[%d]", i)
			var name string
			switch tool.Type {
			case ToolFunction:
				if tool.Function == nil {
					return deepseek.Invalid(field+".function", "is required for a function tool")
				}
				name = tool.Function.Name
				if name == "" {
					return deepseek.Invalid(field+".function.name", "is required")
				}
				if len(name) > 128 {
					return deepseek.Invalid(field+".function.name", "must not exceed 128 characters, got %d", len(name))
				}
				if !toolNamePattern.MatchString(name) {
					return deepseek.Invalid(field+".function.name", "must match %s, got %q", toolNamePattern, name)
				}
			case ToolWebSearch, ToolReadURL:
				if tool.Function != nil {
					return deepseek.Invalid(field+".function", "is not allowed for server-side tools")
				}
				name = string(tool.Type)
			default:
				return deepseek.Invalid(field+".type", "unsupported value %q", tool.Type)
			}
			if seen[name] {
				duplicateField := field + ".type"
				if tool.Type == ToolFunction {
					duplicateField = field + ".function.name"
				}
				return deepseek.Invalid(duplicateField, "duplicate tool name %q", name)
			}
			seen[name] = true
		}
	}
	if r.ToolChoice != nil {
		if err := validateToolChoice(r); err != nil {
			return err
		}
	}
	return nil
}

func validateToolChoice(r *Request) error {
	choice := r.ToolChoice
	if choice.Null {
		return nil
	}
	thinking := r.Thinking != nil && r.Thinking.Type == ThinkingEnabled
	if choice.Function != "" {
		if choice.Mode != "" {
			return deepseek.Invalid("tool_choice", "cannot set both a mode and a function")
		}
		if declaredServerTool(r.Tools, choice.Function) {
			return deepseek.Invalid("tool_choice", "%q cannot be forced; the model decides when to use it", choice.Function)
		}
		if !declaredFunctionTool(r.Tools, choice.Function) {
			return deepseek.Invalid("tool_choice.function.name", "function %q is not declared in tools", choice.Function)
		}
		if thinking {
			return deepseek.Invalid("tool_choice", "a named function is not supported in thinking mode")
		}
		return nil
	}
	switch choice.Mode {
	case ToolChoiceNone, ToolChoiceAuto:
		return nil
	case ToolChoiceRequired:
		if len(r.Tools) == 0 {
			return deepseek.Invalid("tool_choice", `"required" needs at least one tool`)
		}
		if thinking {
			return deepseek.Invalid("tool_choice", "required is not supported in thinking mode")
		}
		return nil
	default:
		return deepseek.Invalid("tool_choice", "unsupported mode %q", choice.Mode)
	}
}

// declaredServerTool reports whether name is a server-side tool enabled by the
// request. The service only rejects forcing a server tool that is declared.
func declaredServerTool(tools []Tool, name string) bool {
	for _, tool := range tools {
		switch tool.Type {
		case ToolWebSearch, ToolReadURL:
			if string(tool.Type) == name {
				return true
			}
		}
	}
	return false
}

func declaredFunctionTool(tools []Tool, name string) bool {
	for _, tool := range tools {
		if tool.Type == ToolFunction && tool.Function != nil && tool.Function.Name == name {
			return true
		}
	}
	return false
}

func validateMessage(index int, m Message) error {
	field := fmt.Sprintf("messages[%d]", index)
	switch m.Role {
	case RoleSystem:
		if m.Content.Null || m.Content.Parts != nil {
			return deepseek.Invalid(field+".content", "system content must be a string")
		}
		if err := validateContent(field+".content", m.Content, false, false); err != nil {
			return err
		}
	case RoleUser:
		if m.ToolCallID != "" {
			return deepseek.Invalid(field+".tool_call_id", "is not valid on a user message")
		}
		if len(m.ToolCalls) > 0 {
			return deepseek.Invalid(field+".tool_calls", "is not valid on a user message")
		}
		if err := validateContent(field+".content", m.Content, false, false); err != nil {
			return err
		}
	case RoleAssistant:
		if m.Content.Parts != nil {
			return deepseek.Invalid(field+".content", "assistant content must be a string or null")
		}
		if m.Content.Null && len(m.ToolCalls) == 0 {
			return deepseek.Invalid(field+".content", "null content requires tool_calls")
		}
		if m.ToolCallID != "" {
			return deepseek.Invalid(field+".tool_call_id", "is not valid on an assistant message")
		}
	case RoleTool:
		if m.ToolCallID == "" {
			return deepseek.Invalid(field+".tool_call_id", "is required")
		}
		if len(m.ToolCalls) > 0 {
			return deepseek.Invalid(field+".tool_calls", "is not valid on a tool message")
		}
		if err := validateContent(field+".content", m.Content, false, false); err != nil {
			return err
		}
	case RoleLatestReminder:
		if m.ToolCallID != "" {
			return deepseek.Invalid(field+".tool_call_id", "is not valid on a latest_reminder message")
		}
		if len(m.ToolCalls) > 0 {
			return deepseek.Invalid(field+".tool_calls", "is not valid on a latest_reminder message")
		}
		if m.Content.Null || m.Content.Parts != nil || strings.TrimSpace(m.Content.Text) == "" {
			return deepseek.Invalid(field+".content", "must be a non-empty <date>,<weekday>,<location>,<platform>,<language> string")
		}
		if err := validateReminderFormat(field+".content", m.Content.Text); err != nil {
			return err
		}
	default:
		return deepseek.Invalid(field+".role", "unsupported value %q", m.Role)
	}
	if m.Prefix != nil && *m.Prefix && m.Role != RoleAssistant {
		return deepseek.Invalid(field+".prefix", "is only valid on an assistant message")
	}
	return nil
}

func validateReminderFormat(field, content string) error {
	fields := strings.Split(content, ",")
	if len(fields) != 5 {
		return deepseek.Invalid(field, "must be <date>,<weekday>,<location>,<platform>,<language>")
	}
	for _, value := range fields {
		if strings.TrimSpace(value) == "" {
			return deepseek.Invalid(field, "must be <date>,<weekday>,<location>,<platform>,<language>")
		}
	}
	return nil
}

func validateContent(field string, content Content, nullable, allowEmpty bool) error {
	if content.Null {
		if nullable {
			return nil
		}
		return deepseek.Invalid(field, "must not be null")
	}
	if content.Parts == nil {
		if !allowEmpty && strings.TrimSpace(content.Text) == "" {
			return deepseek.Invalid(field, "must not be empty")
		}
		return nil
	}
	for i, part := range content.Parts {
		partField := fmt.Sprintf("%s.parts[%d]", field, i)
		if part.Raw != nil {
			continue
		}
		switch part.Type {
		case PartText:
			if part.Text == "" && !allowEmpty {
				return deepseek.Invalid(partField+".text", "is required")
			}
		case PartImage:
			if part.ImageURL == nil || part.ImageURL.URL == "" {
				return deepseek.Invalid(partField+".image_url.url", "is required")
			}
			switch part.ImageURL.Detail {
			case "", "low", "high", "original", "auto":
			default:
				return deepseek.Invalid(partField+".image_url.detail", "unsupported value %q", part.ImageURL.Detail)
			}
		case PartFile:
			if part.File == nil {
				return deepseek.Invalid(partField+".file", "is required")
			}
			count := 0
			if part.File.FileID != "" {
				count++
			}
			if part.File.FileData != "" {
				count++
			}
			if count != 1 {
				return deepseek.Invalid(partField+".file", "requires exactly one of file_id or file_data")
			}
			if part.File.Filename != "" && part.File.FileData == "" {
				return deepseek.Invalid(partField+".file.filename", "is only valid with file_data")
			}
		default:
			return deepseek.Invalid(partField+".type", "unsupported value %q", part.Type)
		}
	}
	return nil
}

func (r *Request) prepared(stream bool) *Request {
	clone := *r
	clone.Stream = new(stream)
	if !stream {
		clone.StreamOptions = nil
		return &clone
	}
	options := StreamOptions{}
	if r.StreamOptions != nil {
		options = *r.StreamOptions
	}
	if options.IncludeUsage == nil {
		options.IncludeUsage = new(true)
	}
	clone.StreamOptions = &options
	return &clone
}
