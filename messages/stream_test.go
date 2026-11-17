package messages

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/http"
	"reflect"
	"testing"

	deepseek "github.com/chimixiong/deepseek-sdk-go"
)

const sseFixture = `: keep-alive

event: message_start
data: {"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","model":"deepseek-flash","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":12,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0,"service_tier":"standard"}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}

event: ping
data: {"type":"ping"}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Plan: "}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig-123"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"citations_delta","citation":{"type":"web_search_result_location","url":"https://example.com","title":"Example","encrypted_index":"","cited_text":"Hello"}}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: content_block_start
data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"Paris\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":2}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"input_tokens":12,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":12,"service_tier":"standard"}}

event: message_stop
data: {"type":"message_stop"}

`

func TestCreateStreamEvents(t *testing.T) {
	client := testClient(t, sseHandler(t))
	var types []EventType
	var signature string
	for event, err := range client.CreateStream(context.Background(), validRequest()) {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		types = append(types, event.Type)
		if event.Type == EventContentBlockDelta && event.Delta != nil && event.Delta.Type == DeltaSignature {
			signature = event.Delta.Signature
		}
	}
	want := []EventType{
		EventMessageStart,
		EventContentBlockStart,
		EventPing,
		EventContentBlockDelta,
		EventContentBlockDelta,
		EventContentBlockStop,
		EventContentBlockStart,
		EventContentBlockDelta,
		EventContentBlockDelta,
		EventContentBlockStop,
		EventContentBlockStart,
		EventContentBlockDelta,
		EventContentBlockDelta,
		EventContentBlockStop,
		EventMessageDelta,
		EventMessageStop,
	}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("event types = %v\nwant %v", types, want)
	}
	if signature != "sig-123" {
		t.Fatalf("signature delta = %q, want sig-123", signature)
	}
}

func TestAggregateStream(t *testing.T) {
	client := testClient(t, sseHandler(t))
	resp, err := Aggregate(client.CreateStream(context.Background(), validRequest()))
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if resp.ID != "msg_01" || resp.Type != MessageType || resp.Role != RoleAssistant || resp.Model != "deepseek-flash" {
		t.Fatalf("message = %#v", resp)
	}
	if len(resp.Content) != 3 {
		t.Fatalf("content = %d blocks, want 3", len(resp.Content))
	}

	thinking := resp.Content[0]
	if thinking.Type != BlockThinking || thinking.Thinking != "Plan: " {
		t.Fatalf("thinking = %#v", thinking)
	}
	if thinking.Signature == nil || *thinking.Signature != "sig-123" {
		t.Fatalf("signature = %#v", thinking.Signature)
	}

	text := resp.Content[1]
	if text.Type != BlockText || text.Text != "Hello" {
		t.Fatalf("text = %#v", text)
	}
	if len(text.Citations) != 1 || text.Citations[0].URL != "https://example.com" {
		t.Fatalf("citations = %#v", text.Citations)
	}

	tool := resp.Content[2]
	if tool.Type != BlockToolUse || tool.ID != "toolu_1" || tool.Name != "get_weather" {
		t.Fatalf("tool_use = %#v", tool)
	}
	var input map[string]any
	if err := json.Unmarshal(tool.Input, &input); err != nil {
		t.Fatalf("tool input %s: %v", tool.Input, err)
	}
	if input["city"] != "Paris" {
		t.Fatalf("tool input = %#v", input)
	}

	if resp.StopReason == nil || *resp.StopReason != StopReasonToolUse {
		t.Fatalf("stop_reason = %#v", resp.StopReason)
	}
	if resp.Usage.InputTokens != 12 || resp.Usage.OutputTokens != 12 {
		t.Fatalf("usage = %#v", resp.Usage)
	}
}

func TestAggregateStreamError(t *testing.T) {
	_, err := Aggregate(eventsSeq(Event{Type: EventError, Error: &StreamError{Type: "overloaded_error", Message: "busy"}}))
	var streamErr *StreamError
	if !errors.As(err, &streamErr) {
		t.Fatalf("error = %#v, want *StreamError", err)
	}
	if streamErr.Type != "overloaded_error" || streamErr.Message != "busy" {
		t.Fatalf("stream error = %#v", streamErr)
	}
}

func TestAggregateIncomplete(t *testing.T) {
	resp, err := Aggregate(eventsSeq(Event{Type: EventMessageStart, Message: &Response{ID: "msg_x"}}))
	if !errors.Is(err, ErrStreamIncomplete) {
		t.Fatalf("error = %v, want ErrStreamIncomplete", err)
	}
	if resp == nil || resp.ID != "msg_x" {
		t.Fatalf("partial response = %#v", resp)
	}
}

func TestAggregateInvalidToolInput(t *testing.T) {
	_, err := Aggregate(eventsSeq(
		Event{Type: EventContentBlockStart, Index: 0, Block: &Block{Type: BlockToolUse, ID: "toolu_1", Name: "lookup"}},
		Event{Type: EventContentBlockDelta, Index: 0, Delta: &Delta{Type: DeltaInputJSON, PartialJSON: "{"}},
		Event{Type: EventContentBlockStop, Index: 0},
		Event{Type: EventMessageStop},
	))
	if err == nil {
		t.Fatal("Aggregate accepted invalid tool input JSON")
	}
}

func TestAggregateMessageDeltaUsage(t *testing.T) {
	resp, err := Aggregate(eventsSeq(
		Event{Type: EventMessageStart, Message: &Response{ID: "msg_y", Usage: Usage{InputTokens: 5, OutputTokens: 1}}},
		Event{Type: EventMessageDelta, StopReason: new(StopReasonEndTurn), Usage: &Usage{InputTokens: 5, OutputTokens: 9}},
		Event{Type: EventMessageStop},
	))
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if resp.Usage.OutputTokens != 9 {
		t.Fatalf("usage = %#v, want the message_delta usage", resp.Usage)
	}
	if resp.StopReason == nil || *resp.StopReason != StopReasonEndTurn {
		t.Fatalf("stop_reason = %#v", resp.StopReason)
	}
}

func TestDecodeErrorFrame(t *testing.T) {
	event, ok, err := decodeEvent(deepseek.SSEEvent{Data: []byte(`{"error":{"message":"timed out"}}`)})
	if err != nil || !ok {
		t.Fatalf("decodeEvent: ok=%v err=%v", ok, err)
	}
	if event.Type != EventError || event.Error == nil || event.Error.Message != "timed out" {
		t.Fatalf("event = %#v", event)
	}
}

func sseHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anthropic/v1/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "secret" {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("Accept = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if doc["stream"] != true {
			t.Errorf("stream = %#v, want true", doc["stream"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, sseFixture)
	}
}

func eventsSeq(events ...Event) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		for _, event := range events {
			if !yield(event, nil) {
				return
			}
		}
	}
}

func TestAggregateMessageDeltaUsageMerge(t *testing.T) {
	webFetches := 1
	resp, err := Aggregate(eventsSeq(
		Event{Type: EventMessageStart, Message: &Response{ID: "msg_z", Usage: Usage{
			InputTokens:          100,
			CacheReadInputTokens: 50,
			OutputTokens:         1,
			OutputTokensDetails:  &OutputTokensDetails{ThinkingTokens: 1},
			ServerToolUse:        &ServerToolUsage{WebFetchRequests: &webFetches},
		}}},
		Event{Type: EventMessageDelta, Usage: &Usage{
			OutputTokens:        20,
			OutputTokensDetails: &OutputTokensDetails{ThinkingTokens: 12},
		}},
		Event{Type: EventMessageDelta, Usage: &Usage{
			OutputTokens: 5,
			ServiceTier:  ServiceTierStandard,
		}},
		Event{Type: EventMessageStop},
	))
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	usage := resp.Usage
	if usage.InputTokens != 100 || usage.CacheReadInputTokens != 50 || usage.OutputTokens != 20 {
		t.Fatalf("usage = %+v", usage)
	}
	if usage.OutputTokensDetails == nil || usage.OutputTokensDetails.ThinkingTokens != 12 {
		t.Fatalf("output_tokens_details = %+v", usage.OutputTokensDetails)
	}
	if usage.ServiceTier != ServiceTierStandard {
		t.Errorf("service_tier = %q", usage.ServiceTier)
	}
	if usage.ServerToolUse == nil || usage.ServerToolUse.WebFetchRequests == nil || *usage.ServerToolUse.WebFetchRequests != 1 {
		t.Fatalf("server_tool_use = %+v", usage.ServerToolUse)
	}
}
