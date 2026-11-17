package messages

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"strings"

	deepseek "github.com/chimixiong/deepseek-sdk-go"
)

// EventType is the SSE event name and JSON type of a stream event.
type EventType string

// Stream event types.
const (
	EventMessageStart      EventType = "message_start"
	EventContentBlockStart EventType = "content_block_start"
	EventContentBlockDelta EventType = "content_block_delta"
	EventContentBlockStop  EventType = "content_block_stop"
	EventMessageDelta      EventType = "message_delta"
	EventMessageStop       EventType = "message_stop"
	EventPing              EventType = "ping"
	EventError             EventType = "error"
)

// DeltaType is the discriminator of a content block delta.
type DeltaType string

// Content block delta types.
const (
	DeltaText      DeltaType = "text_delta"
	DeltaThinking  DeltaType = "thinking_delta"
	DeltaSignature DeltaType = "signature_delta"
	DeltaInputJSON DeltaType = "input_json_delta"
	DeltaCitations DeltaType = "citations_delta"
)

// Delta is one content block increment.
type Delta struct {
	Type DeltaType `json:"type"`
	// Text is appended to the current text block for text_delta.
	Text string `json:"text,omitempty"`
	// Thinking is appended to the current thinking block for thinking_delta.
	Thinking string `json:"thinking,omitempty"`
	// Signature is the completed thinking signature for signature_delta.
	Signature string `json:"signature,omitempty"`
	// PartialJSON is a fragment of a tool input for input_json_delta.
	PartialJSON string `json:"partial_json,omitempty"`
	// Citation is appended to the current text block for citations_delta.
	Citation *Citation `json:"citation,omitempty"`
}

// StreamError is an error delivered in-band. It may arrive as a named "error"
// event or as an unnamed data frame with an error object.
type StreamError struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

// Error implements error.
func (e *StreamError) Error() string {
	if e.Type != "" {
		return "messages: stream error (" + e.Type + "): " + e.Message
	}
	return "messages: stream error: " + e.Message
}

// Event is one stream event. The Type field selects which of the remaining
// fields are populated.
type Event struct {
	Type EventType

	// Message is set for message_start.
	Message *Response
	// Index is the content block index for block events.
	Index int
	// Block is set for content_block_start.
	Block *Block
	// Delta is set for content_block_delta.
	Delta *Delta

	// Usage is the cumulative usage from message_delta.
	Usage *Usage
	// StopReason, StopSequence, StopDetails and Title come from message_delta.
	StopReason   *StopReason
	StopSequence *string
	StopDetails  *StopDetails
	Title        *string

	// Error is set for error events.
	Error *StreamError
	// Raw is the event payload as the server sent it.
	Raw json.RawMessage
}

func decodeEvent(raw deepseek.SSEEvent) (Event, bool, error) {
	data := bytes.TrimSpace(raw.Data)
	if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
		return Event{}, false, nil
	}
	var head struct {
		Type  EventType    `json:"type"`
		Error *StreamError `json:"error"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return Event{}, false, fmt.Errorf("messages: decode event %q: %w", raw.Event, err)
	}
	event := Event{Type: head.Type, Error: head.Error, Raw: append(json.RawMessage(nil), data...)}
	if event.Type == "" {
		event.Type = EventType(raw.Event)
	}
	if event.Type == "" && head.Error != nil {
		event.Type = EventError
	}
	switch event.Type {
	case EventMessageStart:
		var wire struct {
			Message *Response `json:"message"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return Event{}, false, fmt.Errorf("messages: decode message_start: %w", err)
		}
		event.Message = wire.Message
	case EventContentBlockStart:
		var wire struct {
			Index int    `json:"index"`
			Block *Block `json:"content_block"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return Event{}, false, fmt.Errorf("messages: decode content_block_start: %w", err)
		}
		event.Index, event.Block = wire.Index, wire.Block
	case EventContentBlockDelta:
		var wire struct {
			Index int    `json:"index"`
			Delta *Delta `json:"delta"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return Event{}, false, fmt.Errorf("messages: decode content_block_delta: %w", err)
		}
		event.Index, event.Delta = wire.Index, wire.Delta
	case EventContentBlockStop:
		var wire struct {
			Index int `json:"index"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return Event{}, false, fmt.Errorf("messages: decode content_block_stop: %w", err)
		}
		event.Index = wire.Index
	case EventMessageDelta:
		var wire struct {
			Delta struct {
				StopReason   *StopReason  `json:"stop_reason"`
				StopSequence *string      `json:"stop_sequence"`
				StopDetails  *StopDetails `json:"stop_details"`
				Title        *string      `json:"title"`
			} `json:"delta"`
			Usage *Usage `json:"usage"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return Event{}, false, fmt.Errorf("messages: decode message_delta: %w", err)
		}
		event.StopReason = wire.Delta.StopReason
		event.StopSequence = wire.Delta.StopSequence
		event.StopDetails = wire.Delta.StopDetails
		event.Title = wire.Delta.Title
		event.Usage = wire.Usage
	}
	return event, true, nil
}

// ErrStreamIncomplete reports a stream that closed before message_stop.
var ErrStreamIncomplete = errors.New("messages: stream ended before message_stop")

// Aggregate consumes a stream and assembles the final message. Text, thinking,
// signatures, citations and tool inputs are accumulated per content block;
// message_delta counters are merged into the message_start usage (see
// mergeUsage), and the last message_delta stop reason wins. The partial message
// is returned together with any error, including ErrStreamIncomplete.
func Aggregate(seq iter.Seq2[Event, error]) (*Response, error) {
	acc := &accumulator{}
	for event, err := range seq {
		if err != nil {
			return acc.response(), err
		}
		if err := acc.apply(event); err != nil {
			return acc.response(), err
		}
	}
	if err := acc.finishPending(); err != nil {
		return acc.response(), err
	}
	if !acc.stopped {
		return acc.response(), ErrStreamIncomplete
	}
	return acc.response(), nil
}

type accumulator struct {
	message  *Response
	blocks   []*Block
	partials map[int]*strings.Builder
	stopped  bool
}

func (a *accumulator) apply(event Event) error {
	switch event.Type {
	case EventMessageStart:
		if event.Message == nil {
			return errors.New("messages: message_start event without a message")
		}
		message := *event.Message
		a.message = &message
		a.blocks = nil
		a.partials = nil
		a.stopped = false
	case EventContentBlockStart:
		if event.Block == nil {
			return errors.New("messages: content_block_start event without a content_block")
		}
		if a.message == nil {
			a.message = &Response{}
		}
		block := *event.Block
		a.setBlock(event.Index, &block)
	case EventContentBlockDelta:
		if event.Delta == nil {
			return errors.New("messages: content_block_delta event without a delta")
		}
		block, err := a.block(event.Index)
		if err != nil {
			return err
		}
		switch event.Delta.Type {
		case DeltaText:
			block.Text += event.Delta.Text
		case DeltaThinking:
			block.Thinking += event.Delta.Thinking
		case DeltaSignature:
			block.Signature = new(event.Delta.Signature)
		case DeltaInputJSON:
			a.appendPartial(event.Index, event.Delta.PartialJSON)
		case DeltaCitations:
			if event.Delta.Citation != nil {
				block.Citations = append(block.Citations, *event.Delta.Citation)
			}
		}
	case EventContentBlockStop:
		return a.finishBlock(event.Index)
	case EventMessageDelta:
		if a.message == nil {
			a.message = &Response{}
		}
		if event.Usage != nil {
			mergeUsage(&a.message.Usage, event.Usage)
		}
		if event.StopReason != nil {
			a.message.StopReason = new(*event.StopReason)
		}
		if event.StopSequence != nil {
			a.message.StopSequence = new(*event.StopSequence)
		}
		a.message.StopDetails = event.StopDetails
		if event.Title != nil {
			a.message.Title = new(*event.Title)
		}
	case EventMessageStop:
		a.stopped = true
	case EventError:
		if event.Error != nil {
			return event.Error
		}
		return errors.New("messages: stream error")
	}
	return nil
}

func (a *accumulator) setBlock(index int, block *Block) {
	for len(a.blocks) <= index {
		a.blocks = append(a.blocks, nil)
	}
	a.blocks[index] = block
}

func (a *accumulator) block(index int) (*Block, error) {
	if index < 0 || index >= len(a.blocks) || a.blocks[index] == nil {
		return nil, fmt.Errorf("messages: delta for content block %d, which has not started", index)
	}
	return a.blocks[index], nil
}

func (a *accumulator) appendPartial(index int, fragment string) {
	if a.partials == nil {
		a.partials = make(map[int]*strings.Builder)
	}
	builder, ok := a.partials[index]
	if !ok {
		builder = &strings.Builder{}
		a.partials[index] = builder
	}
	builder.WriteString(fragment)
}

func (a *accumulator) finishBlock(index int) error {
	builder, ok := a.partials[index]
	if !ok {
		return nil
	}
	delete(a.partials, index)
	if builder.Len() == 0 {
		return nil
	}
	block, err := a.block(index)
	if err != nil {
		return err
	}
	raw := []byte(builder.String())
	if !json.Valid(raw) {
		return fmt.Errorf("messages: tool input for content block %d is not valid JSON", index)
	}
	block.Input = append(json.RawMessage(nil), raw...)
	return nil
}

func (a *accumulator) finishPending() error {
	for index := range a.partials {
		if err := a.finishBlock(index); err != nil {
			return err
		}
	}
	return nil
}

func (a *accumulator) response() *Response {
	if a.message == nil {
		return nil
	}
	message := *a.message
	message.Content = make([]Block, 0, len(a.blocks))
	for _, block := range a.blocks {
		if block != nil {
			message.Content = append(message.Content, *block)
		}
	}
	return &message
}
