package chat

import (
	"encoding/json"
	"sort"

	"github.com/chimixiong/deepseek-sdk-go"
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
	// Error is set on the last chunk when generation fails after the stream
	// started. Such a chunk carries usage and has no choices; no [DONE] follows.
	Error *deepseek.ResponseError `json:"error,omitempty"`
}

// ChunkChoice is one streamed candidate.
type ChunkChoice struct {
	Index        int             `json:"index"`
	Delta        Delta           `json:"delta"`
	FinishReason FinishReason    `json:"finish_reason,omitempty"`
	Logprobs     *ChoiceLogprobs `json:"logprobs,omitempty"`
	// ServerToolEvent is the DeepSeek extension describing server-side web
	// tool progress. One chunk carries at most one event; "type" is
	// step_reasoning_done, call_started or call_completed.
	ServerToolEvent json.RawMessage `json:"server_tool_event,omitempty"`
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
// fragments are concatenated per index. State is kept per choice, and the
// struct may be copied before Add: no field contains a non-zero
// strings.Builder.
type StreamAccumulator struct {
	id          string
	object      string
	created     int64
	model       string
	fingerprint string

	choices map[int]*streamChoice
	order   []int

	usage         *Usage
	responseError *deepseek.ResponseError
}

type streamChoice struct {
	content   []byte
	reasoning []byte

	toolCalls map[int]*ToolCall
	toolOrder []int

	finish          FinishReason
	annotations     []json.RawMessage
	serverToolCalls []json.RawMessage
	title           string
	refusal         string
	logprobs        *ChoiceLogprobs
}

func (a *StreamAccumulator) choice(index int) *streamChoice {
	if a.choices == nil {
		a.choices = make(map[int]*streamChoice)
	}
	choice, ok := a.choices[index]
	if !ok {
		choice = &streamChoice{}
		a.choices[index] = choice
		a.order = append(a.order, index)
	}
	return choice
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
	if chunk.Error != nil {
		a.responseError = chunk.Error
	}
	for _, fragmentChoice := range chunk.Choices {
		choice := a.choice(fragmentChoice.Index)
		if fragmentChoice.FinishReason != "" {
			choice.finish = fragmentChoice.FinishReason
		}
		delta := fragmentChoice.Delta
		if delta.Content != nil {
			choice.content = append(choice.content, *delta.Content...)
		}
		if delta.ReasoningContent != nil {
			choice.reasoning = append(choice.reasoning, *delta.ReasoningContent...)
		}
		if len(delta.Annotations) > 0 {
			choice.annotations = append(choice.annotations, delta.Annotations...)
		}
		if len(delta.ServerToolCalls) > 0 {
			choice.serverToolCalls = append(choice.serverToolCalls, delta.ServerToolCalls...)
		}
		// title and refusal are documented to appear once, in the final chunk
		// of a stream; appending is equivalent to assignment for a compliant
		// server and still aggregates correctly if one ever fragments them.
		choice.title += delta.Title
		choice.refusal += delta.Refusal
		if fragmentChoice.Logprobs != nil {
			if choice.logprobs == nil {
				choice.logprobs = &ChoiceLogprobs{}
			}
			choice.logprobs.Content = append(choice.logprobs.Content, fragmentChoice.Logprobs.Content...)
			choice.logprobs.ReasoningContent = append(choice.logprobs.ReasoningContent, fragmentChoice.Logprobs.ReasoningContent...)
		}
		for _, fragment := range delta.ToolCalls {
			if choice.toolCalls == nil {
				choice.toolCalls = make(map[int]*ToolCall)
			}
			call, ok := choice.toolCalls[fragment.Index]
			if !ok {
				call = &ToolCall{Type: "function"}
				choice.toolCalls[fragment.Index] = call
				choice.toolOrder = append(choice.toolOrder, fragment.Index)
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
}

// Response returns the assembled non-streaming response.
func (a *StreamAccumulator) Response() *Response {
	order := append([]int(nil), a.order...)
	sort.Ints(order)
	choices := make([]Choice, 0, len(order))
	for _, index := range order {
		choice := a.choices[index]
		message := Message{
			Role:            RoleAssistant,
			Content:         TextContent(string(choice.content)),
			Annotations:     choice.annotations,
			ServerToolCalls: choice.serverToolCalls,
			Title:           choice.title,
			Refusal:         choice.refusal,
		}
		if len(choice.reasoning) > 0 {
			reasoning := string(choice.reasoning)
			message.ReasoningContent = &reasoning
		}
		if len(choice.toolOrder) > 0 {
			sort.Ints(choice.toolOrder)
			for _, callIndex := range choice.toolOrder {
				message.ToolCalls = append(message.ToolCalls, *choice.toolCalls[callIndex])
			}
		}
		choices = append(choices, Choice{
			Index:        index,
			Message:      message,
			FinishReason: choice.finish,
			Logprobs:     choice.logprobs,
		})
	}
	return &Response{
		ID:                a.id,
		Object:            a.object,
		Created:           a.created,
		Model:             a.model,
		SystemFingerprint: a.fingerprint,
		Choices:           choices,
		Usage:             a.usage,
		Error:             a.responseError,
	}
}
