package responses

import (
	"encoding/json"
	"errors"
	"iter"

	"github.com/chimixiong/deepseek-sdk-go"
)

// EventType is a semantic streaming event name.
type EventType string

const (
	EventCreated          EventType = "response.created"
	EventInProgress       EventType = "response.in_progress"
	EventOutputItemAdded  EventType = "response.output_item.added"
	EventOutputItemDone   EventType = "response.output_item.done"
	EventContentPartAdded EventType = "response.content_part.added"
	EventContentPartDone  EventType = "response.content_part.done"

	EventOutputTextDelta    EventType = "response.output_text.delta"
	EventOutputTextDone     EventType = "response.output_text.done"
	EventReasoningTextDelta EventType = "response.reasoning_text.delta"
	EventReasoningTextDone  EventType = "response.reasoning_text.done"

	EventFunctionCallArgumentsDelta EventType = "response.function_call_arguments.delta"
	EventFunctionCallArgumentsDone  EventType = "response.function_call_arguments.done"
	EventCustomToolCallInputDelta   EventType = "response.custom_tool_call_input.delta"
	EventCustomToolCallInputDone    EventType = "response.custom_tool_call_input.done"

	EventWebSearchCallInProgress EventType = "response.web_search_call.in_progress"
	EventWebSearchCallSearching  EventType = "response.web_search_call.searching"
	EventWebSearchCallCompleted  EventType = "response.web_search_call.completed"

	// EventOutputTextAnnotationAdded carries a citation annotation for an
	// output_text part. It is emitted by the web-search tool.
	EventOutputTextAnnotationAdded EventType = "response.output_text.annotation.added"

	EventCompleted  EventType = "response.completed"
	EventIncomplete EventType = "response.incomplete"
	EventFailed     EventType = "response.failed"
)

// Terminal reports whether the event ends the stream.
func (t EventType) Terminal() bool {
	switch t {
	case EventCompleted, EventIncomplete, EventFailed:
		return true
	}
	return false
}

// Event is one semantic streaming event. Fields that do not apply to the event
// type stay empty. Unknown event types and unmodelled fields are preserved.
type Event struct {
	Type EventType `json:"type"`
	// SequenceNumber is the server's event sequence number. It is passed
	// through verbatim; the SDK does not reject duplicates or gaps. Consumers
	// that need stream-integrity checks must track it themselves.
	SequenceNumber int `json:"sequence_number,omitempty"`

	// Response is set on created, in_progress and the terminal events.
	Response *Response `json:"response,omitempty"`

	// Item is set on output_item events.
	Item *OutputItem `json:"item,omitempty"`
	// ItemID, OutputIndex and ContentIndex locate the affected item or part.
	ItemID       string `json:"item_id,omitempty"`
	OutputIndex  int    `json:"output_index,omitempty"`
	ContentIndex int    `json:"content_index,omitempty"`

	// Part is the added or completed content part.
	Part *ContentPart `json:"part,omitempty"`
	// Delta is an incremental text, argument or input fragment.
	Delta string `json:"delta,omitempty"`
	// Text is the complete text of output_text.done and reasoning_text.done.
	Text string `json:"text,omitempty"`
	// Arguments is the complete argument string of
	// function_call_arguments.done.
	Arguments string `json:"arguments,omitempty"`
	// Input is the complete custom tool input of custom_tool_call_input.done.
	Input string `json:"input,omitempty"`
	// Logprobs accompanies output_text events when top_logprobs was requested.
	Logprobs []json.RawMessage `json:"logprobs,omitempty"`
	// Annotation accompanies output_text.annotation.added.
	Annotation json.RawMessage `json:"annotation,omitempty"`

	// Extra holds event fields this release does not model.
	Extra map[string]json.RawMessage `json:"-"`
}

type eventAlias Event

// MarshalJSON injects Extra.
func (e Event) MarshalJSON() ([]byte, error) {
	return deepseek.MarshalWithExtra(eventAlias(e), e.Extra)
}

// UnmarshalJSON keeps fields this release does not model.
func (e *Event) UnmarshalJSON(data []byte) error {
	var wire eventAlias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*e = Event(wire)
	extra, err := splitExtra(data, Event{})
	if err != nil {
		return err
	}
	e.Extra = extra
	return nil
}

// Terminal reports whether the event ends the stream.
func (e Event) Terminal() bool { return e.Type.Terminal() }

// Accumulate drains a stream and returns the response carried by the terminal
// event. It fails when the stream ends without response.completed,
// response.incomplete or response.failed. A successful terminal event ends the
// read immediately, so a connection reset after it cannot discard the response.
func Accumulate(seq iter.Seq2[Event, error]) (*Response, error) {
	if seq == nil {
		return nil, deepseek.Invalid("stream", "must not be nil")
	}
	var final *Response
	for event, err := range seq {
		if err != nil {
			return final, err
		}
		if event.Terminal() {
			final = event.Response
			break
		}
	}
	if final == nil {
		return nil, errors.New("deepseek/responses: stream ended without a terminal response event")
	}
	if err := final.failureError(); err != nil {
		return final, err
	}
	return final, nil
}
