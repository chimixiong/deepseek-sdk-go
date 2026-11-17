package chat

import (
	"encoding/json"
	"sort"
	"strings"
)

// Chunk is one streamed chat.completion.chunk.
type Chunk struct {
	ID                string        `json:"id"`
	Object            string        `json:"object"`
	Created           int64         `json:"created"`
	Model             string        `json:"model"`
	SystemFingerprint string        `json:"system_fingerprint"`
	Choices           []ChunkChoice `json:"choices"`
	Usage             *Usage        `json:"usage,omitempty"`
}

// ChunkChoice is one streamed candidate.
type ChunkChoice struct {
	Index        int             `json:"index"`
	Delta        Delta           `json:"delta"`
	FinishReason FinishReason    `json:"finish_reason,omitempty"`
	Logprobs     *ChoiceLogprobs `json:"logprobs,omitempty"`
}

// Delta is an incremental message. Content and ReasoningContent are pointers so
// an absent field differs from an empty string.
type Delta struct {
	Role             Role              `json:"role,omitempty"`
	Content          *string           `json:"content,omitempty"`
	ReasoningContent *string           `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCallDelta   `json:"tool_calls,omitempty"`
	Annotations      []json.RawMessage `json:"annotations,omitempty"`
	ServerToolCalls  []json.RawMessage `json:"server_tool_calls,omitempty"`
	Title            string            `json:"title,omitempty"`
	Refusal          string            `json:"refusal,omitempty"`
}

// StreamAccumulator assembles chunks into one Response. Tool-call argument
// fragments are concatenated per index.
type StreamAccumulator struct {
	id          string
	object      string
	created     int64
	model       string
	fingerprint string

	content   strings.Builder
	reasoning strings.Builder

	toolCalls map[int]*ToolCall
	toolOrder []int

	finish          FinishReason
	usage           *Usage
	annotations     []json.RawMessage
	serverToolCalls []json.RawMessage
	title           string
	refusal         string
	logprobs        *ChoiceLogprobs
}

// Add merges one chunk.
func (a *StreamAccumulator) Add(chunk Chunk) {
	if a.id == "" {
		a.id = chunk.ID
	}
	if a.object == "" {
		a.object = chunk.Object
	}
	if a.created == 0 {
		a.created = chunk.Created
	}
	if a.model == "" {
		a.model = chunk.Model
	}
	if a.fingerprint == "" {
		a.fingerprint = chunk.SystemFingerprint
	}
	if chunk.Usage != nil {
		a.usage = chunk.Usage
	}
	if len(chunk.Choices) == 0 {
		return
	}
	choice := chunk.Choices[0]
	if choice.FinishReason != "" {
		a.finish = choice.FinishReason
	}
	delta := choice.Delta
	if delta.Content != nil {
		a.content.WriteString(*delta.Content)
	}
	if delta.ReasoningContent != nil {
		a.reasoning.WriteString(*delta.ReasoningContent)
	}
	if len(delta.Annotations) > 0 {
		a.annotations = append(a.annotations, delta.Annotations...)
	}
	if len(delta.ServerToolCalls) > 0 {
		a.serverToolCalls = append(a.serverToolCalls, delta.ServerToolCalls...)
	}
	// title and refusal are documented to appear once, in the final chunk of a
	// stream; appending is equivalent to assignment for a compliant server and
	// still aggregates correctly if one ever fragments them.
	a.title += delta.Title
	a.refusal += delta.Refusal
	if choice.Logprobs != nil {
		if a.logprobs == nil {
			a.logprobs = &ChoiceLogprobs{}
		}
		a.logprobs.Content = append(a.logprobs.Content, choice.Logprobs.Content...)
		a.logprobs.ReasoningContent = append(a.logprobs.ReasoningContent, choice.Logprobs.ReasoningContent...)
	}
	for _, fragment := range delta.ToolCalls {
		if a.toolCalls == nil {
			a.toolCalls = make(map[int]*ToolCall)
		}
		call, ok := a.toolCalls[fragment.Index]
		if !ok {
			call = &ToolCall{Type: "function"}
			a.toolCalls[fragment.Index] = call
			a.toolOrder = append(a.toolOrder, fragment.Index)
		}
		if fragment.ID != "" {
			call.ID = fragment.ID
		}
		if fragment.Type != "" {
			call.Type = fragment.Type
		}
		if fragment.Function.Name != "" {
			call.Function.Name = fragment.Function.Name
		}
		call.Function.Arguments += fragment.Function.Arguments
	}
}

// Response returns the assembled non-streaming response.
func (a *StreamAccumulator) Response() *Response {
	message := Message{
		Role:            RoleAssistant,
		Content:         TextContent(a.content.String()),
		Annotations:     a.annotations,
		ServerToolCalls: a.serverToolCalls,
		Title:           a.title,
		Refusal:         a.refusal,
	}
	if a.reasoning.Len() > 0 {
		reasoning := a.reasoning.String()
		message.ReasoningContent = &reasoning
	}
	if len(a.toolOrder) > 0 {
		sort.Ints(a.toolOrder)
		for _, index := range a.toolOrder {
			message.ToolCalls = append(message.ToolCalls, *a.toolCalls[index])
		}
	}
	return &Response{
		ID:                a.id,
		Object:            a.object,
		Created:           a.created,
		Model:             a.model,
		SystemFingerprint: a.fingerprint,
		Choices: []Choice{{
			Index:        0,
			Message:      message,
			FinishReason: a.finish,
			Logprobs:     a.logprobs,
		}},
		Usage: a.usage,
	}
}
