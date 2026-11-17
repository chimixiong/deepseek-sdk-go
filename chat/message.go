package chat

import (
	"encoding/json"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Role is a message author.
type Role string

const (
	RoleSystem         Role = "system"
	RoleUser           Role = "user"
	RoleAssistant      Role = "assistant"
	RoleTool           Role = "tool"
	RoleLatestReminder Role = "latest_reminder"
)

// Message is one request or response message. Fields that do not apply to the
// message role stay empty.
type Message struct {
	Role             Role              `json:"role"`
	Content          Content           `json:"content"`
	Name             string            `json:"name,omitempty"`
	Prefix           *bool             `json:"prefix,omitempty"`
	ReasoningContent *string           `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall        `json:"tool_calls,omitempty"`
	ToolCallID       string            `json:"tool_call_id,omitempty"`
	ServerToolCalls  []json.RawMessage `json:"server_tool_calls,omitempty"`
	Annotations      []json.RawMessage `json:"annotations,omitempty"`
	Title            string            `json:"title,omitempty"`
	Refusal          string            `json:"refusal,omitempty"`
	// Extra merges additional top-level fields into the message object.
	Extra map[string]json.RawMessage `json:"-"`
}

type messageAlias Message

// MarshalJSON injects Extra.
func (m Message) MarshalJSON() ([]byte, error) {
	return deepseek.MarshalWithExtra(messageAlias(m), m.Extra)
}

// SystemMessage builds a system message.
func SystemMessage(content string) Message {
	return Message{Role: RoleSystem, Content: TextContent(content)}
}

// UserMessage builds a text-only user message.
func UserMessage(content string) Message {
	return Message{Role: RoleUser, Content: TextContent(content)}
}

// UserContentMessage builds a user message from content blocks.
func UserContentMessage(parts ...ContentPart) Message {
	return Message{Role: RoleUser, Content: PartsContent(parts...)}
}

// AssistantMessage builds a text-only assistant message.
func AssistantMessage(content string) Message {
	return Message{Role: RoleAssistant, Content: TextContent(content)}
}

// AssistantToolCalls builds an assistant message that carries tool calls and,
// when tools are in play, the reasoning content that must be echoed back.
func AssistantToolCalls(reasoning string, calls ...ToolCall) Message {
	var reasoningPtr *string
	if reasoning != "" {
		reasoningPtr = &reasoning
	}
	return Message{
		Role:             RoleAssistant,
		Content:          TextContent(""),
		ReasoningContent: reasoningPtr,
		ToolCalls:        calls,
	}
}

// ToolMessage builds a tool result message.
func ToolMessage(toolCallID string, content Content) Message {
	return Message{Role: RoleTool, ToolCallID: toolCallID, Content: content}
}

// LatestReminderMessage builds a latest_reminder message. Content must be
// "<date>,<weekday>,<location>,<platform>,<language>" and sit immediately
// before its user message.
func LatestReminderMessage(content string) Message {
	return Message{Role: RoleLatestReminder, Content: TextContent(content)}
}
