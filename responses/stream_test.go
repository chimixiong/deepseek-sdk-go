package responses_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go"
	"github.com/chimixiong/deepseek-sdk-go/responses"
)

type sseEvent struct {
	event string
	data  string
}

func eventJSON(name string, fields ...string) sseEvent {
	data := `{"type":` + strconv.Quote(name)
	for _, field := range fields {
		data += "," + field
	}
	return sseEvent{event: name, data: data + "}"}
}

func jsonString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func streamingClient(t *testing.T, events []sseEvent) *responses.Client {
	t.Helper()
	return newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream *bool `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode stream request: %v", err)
			return
		}
		if body.Stream == nil || !*body.Stream {
			t.Errorf("stream = %v, want true", body.Stream)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range events {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.event, event.data)
		}
	})
}

const streamCreatedResponse = `{"id":"resp_1","object":"response","created_at":1793084908,"status":"in_progress","model":"deepseek-flash","output":[]}`

const streamCompletedResponse = `{"id":"resp_1","object":"response","created_at":1793084908,"status":"completed","completed_at":1793084909,"model":"deepseek-flash","output":[{"type":"message","id":"msg_1","status":"completed","role":"assistant","content":[{"type":"output_text","annotations":[],"logprobs":[],"text":"Hello world"}],"phase":"final_answer"}],"usage":{"input_tokens":7,"input_tokens_details":{"cached_tokens":0},"output_tokens":2,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":9}}`

func completedStream() []sseEvent {
	return []sseEvent{
		eventJSON(string(responses.EventCreated), `"response":`+streamCreatedResponse, `"sequence_number":0`),
		eventJSON(string(responses.EventInProgress), `"response":`+streamCreatedResponse, `"sequence_number":1`),

		eventJSON(string(responses.EventOutputItemAdded),
			`"item":{"type":"reasoning","id":"rs_1","status":"in_progress","content":[]}`,
			`"output_index":0`, `"sequence_number":2`),
		eventJSON(string(responses.EventContentPartAdded),
			`"content_index":0`, `"item_id":"rs_1"`, `"output_index":0`,
			`"part":{"type":"reasoning_text","text":""}`, `"sequence_number":3`),
		eventJSON(string(responses.EventReasoningTextDelta),
			`"content_index":0`, `"delta":"Think"`, `"item_id":"rs_1"`, `"output_index":0`,
			`"sequence_number":4`),
		eventJSON(string(responses.EventReasoningTextDone),
			`"content_index":0`, `"item_id":"rs_1"`, `"output_index":0`,
			`"sequence_number":5`, `"text":"Think hard."`),
		eventJSON(string(responses.EventContentPartDone),
			`"content_index":0`, `"item_id":"rs_1"`, `"output_index":0`,
			`"part":{"type":"reasoning_text","text":"Think hard."}`, `"sequence_number":6`),
		eventJSON(string(responses.EventOutputItemDone),
			`"item":{"type":"reasoning","id":"rs_1","status":"completed","content":[{"type":"reasoning_text","text":"Think hard."}]}`,
			`"output_index":0`, `"sequence_number":7`),

		eventJSON(string(responses.EventOutputItemAdded),
			`"item":{"type":"message","id":"msg_1","status":"in_progress","content":[],"role":"assistant"}`,
			`"output_index":1`, `"sequence_number":8`),
		eventJSON(string(responses.EventContentPartAdded),
			`"content_index":0`, `"item_id":"msg_1"`, `"output_index":1`,
			`"part":{"type":"output_text","annotations":[],"logprobs":[],"text":""}`, `"sequence_number":9`),
		eventJSON(string(responses.EventOutputTextDelta),
			`"content_index":0`, `"delta":"Hello"`, `"item_id":"msg_1"`, `"output_index":1`,
			`"logprobs":[]`, `"sequence_number":10`),
		eventJSON(string(responses.EventOutputTextDelta),
			`"content_index":0`, `"delta":" world"`, `"item_id":"msg_1"`, `"output_index":1`,
			`"sequence_number":11`),
		eventJSON(string(responses.EventOutputTextAnnotationAdded),
			`"annotation":{"type":"url_citation","url":"https://example.com","start_index":0,"end_index":5}`,
			`"content_index":0`, `"item_id":"msg_1"`, `"output_index":1`, `"sequence_number":12`),
		eventJSON(string(responses.EventOutputTextDone),
			`"content_index":0`, `"item_id":"msg_1"`, `"output_index":1`,
			`"sequence_number":13`, `"text":"Hello world"`),
		eventJSON(string(responses.EventContentPartDone),
			`"content_index":0`, `"item_id":"msg_1"`, `"output_index":1`,
			`"part":{"type":"output_text","annotations":[],"logprobs":[],"text":"Hello world"}`, `"sequence_number":14`),
		eventJSON(string(responses.EventOutputItemDone),
			`"item":{"type":"message","id":"msg_1","status":"completed","content":[{"type":"output_text","annotations":[],"logprobs":[],"text":"Hello world"}],"phase":"final_answer","role":"assistant"}`,
			`"output_index":1`, `"sequence_number":15`),

		eventJSON(string(responses.EventOutputItemAdded),
			`"item":{"type":"function_call","id":"fc_1","status":"in_progress","call_id":"call_1","name":"get_weather","arguments":""}`,
			`"output_index":2`, `"sequence_number":16`),
		eventJSON(string(responses.EventFunctionCallArgumentsDelta),
			`"delta":`+jsonString(`{"city":`), `"item_id":"fc_1"`, `"output_index":2`, `"sequence_number":17`),
		eventJSON(string(responses.EventFunctionCallArgumentsDelta),
			`"delta":`+jsonString(`"Paris"}`), `"item_id":"fc_1"`, `"output_index":2`, `"sequence_number":18`),
		eventJSON(string(responses.EventFunctionCallArgumentsDone),
			`"arguments":`+jsonString(`{"city":"Paris"}`), `"item_id":"fc_1"`, `"output_index":2`, `"sequence_number":19`),
		eventJSON(string(responses.EventOutputItemDone),
			`"item":{"type":"function_call","id":"fc_1","status":"completed","call_id":"call_1","name":"get_weather","arguments":`+jsonString(`{"city":"Paris"}`)+`}`,
			`"output_index":2`, `"sequence_number":20`),

		eventJSON(string(responses.EventOutputItemAdded),
			`"item":{"type":"custom_tool_call","id":"ctc_1","status":"in_progress","call_id":"call_2","name":"apply_patch","input":""}`,
			`"output_index":3`, `"sequence_number":21`),
		eventJSON(string(responses.EventCustomToolCallInputDelta),
			`"delta":`+jsonString("*** Begin"), `"item_id":"ctc_1"`, `"output_index":3`, `"sequence_number":22`),
		eventJSON(string(responses.EventCustomToolCallInputDone),
			`"input":`+jsonString("*** Begin Patch"), `"item_id":"ctc_1"`, `"output_index":3`, `"sequence_number":23`),
		eventJSON(string(responses.EventOutputItemDone),
			`"item":{"type":"custom_tool_call","id":"ctc_1","status":"completed","call_id":"call_2","name":"apply_patch","input":`+jsonString("*** Begin Patch")+`}`,
			`"output_index":3`, `"sequence_number":24`),

		eventJSON(string(responses.EventOutputItemAdded),
			`"item":{"type":"web_search_call","id":"ws_1","status":"in_progress","kind":"search","action":{"type":"search","queries":["paris weather"],"sources":[{"type":"url","url":"https://example.com","title":"Weather","snippet":"Sunny"}]}}`,
			`"output_index":4`, `"sequence_number":25`),
		eventJSON(string(responses.EventWebSearchCallInProgress),
			`"item_id":"ws_1"`, `"output_index":4`, `"sequence_number":26`),
		eventJSON(string(responses.EventWebSearchCallSearching),
			`"item_id":"ws_1"`, `"output_index":4`, `"sequence_number":27`),
		eventJSON(string(responses.EventWebSearchCallCompleted),
			`"item_id":"ws_1"`, `"output_index":4`, `"sequence_number":28`),
		eventJSON(string(responses.EventOutputItemDone),
			`"item":{"type":"web_search_call","id":"ws_1","status":"completed","kind":"search","action":{"type":"search","queries":["paris weather"],"sources":[{"type":"url","url":"https://example.com","title":"Weather","snippet":"Sunny"}]},"page":{"url":"https://example.com","title":"Example Domain","published_at":null},"signature":"c2ln"}`,
			`"output_index":4`, `"sequence_number":29`),

		eventJSON(string(responses.EventCompleted),
			`"response":`+streamCompletedResponse, `"sequence_number":30`),
	}
}

func TestCreateStream(t *testing.T) {
	events := completedStream()
	client := streamingClient(t, events)

	var got []responses.Event
	for event, err := range client.CreateStream(context.Background(), &responses.Request{
		Model: "deepseek-flash",
		Input: responses.StringInput("Hi, how are you?"),
	}) {
		if err != nil {
			t.Fatalf("CreateStream: %v", err)
		}
		got = append(got, event)
	}
	if len(got) != len(events) {
		t.Fatalf("events = %d, want %d", len(got), len(events))
	}
	for i, event := range got {
		if want := responses.EventType(events[i].event); event.Type != want {
			t.Errorf("event %d type = %q, want %q", i, event.Type, want)
		}
	}

	var (
		textDeltas      strings.Builder
		reasoningDone   string
		argumentDeltas  strings.Builder
		argumentsDone   string
		customDeltas    strings.Builder
		customDone      string
		sawAnnotation   bool
		sawSearchAdded  bool
		sawSearchStates int
	)
	for _, event := range got {
		switch event.Type {
		case responses.EventReasoningTextDelta:
			if event.ItemID != "rs_1" || event.OutputIndex != 0 || event.ContentIndex != 0 {
				t.Errorf("reasoning delta location = %+v", event)
			}
			if event.Delta != "Think" {
				t.Errorf("reasoning delta = %q", event.Delta)
			}
		case responses.EventReasoningTextDone:
			reasoningDone = event.Text
		case responses.EventOutputTextDelta:
			textDeltas.WriteString(event.Delta)
		case responses.EventOutputTextDone:
			if event.Text != "Hello world" {
				t.Errorf("output text done = %q", event.Text)
			}
		case responses.EventOutputTextAnnotationAdded:
			sawAnnotation = strings.Contains(string(event.Annotation), "url_citation")
		case responses.EventFunctionCallArgumentsDelta:
			argumentDeltas.WriteString(event.Delta)
		case responses.EventFunctionCallArgumentsDone:
			argumentsDone = event.Arguments
		case responses.EventCustomToolCallInputDelta:
			customDeltas.WriteString(event.Delta)
		case responses.EventCustomToolCallInputDone:
			customDone = event.Input
		case responses.EventOutputItemAdded:
			if event.Item == nil {
				t.Fatal("output_item.added without item")
			}
			if event.Item.Type == responses.ItemWebSearchCall {
				sawSearchAdded = event.Item.Action != nil && event.Item.Action.Type == responses.ActionSearch
			}
		case responses.EventWebSearchCallInProgress, responses.EventWebSearchCallSearching, responses.EventWebSearchCallCompleted:
			sawSearchStates++
			if event.ItemID != "ws_1" {
				t.Errorf("web search state item = %q", event.ItemID)
			}
		case responses.EventCompleted:
			if event.Response == nil || event.Response.Status != responses.StatusCompleted {
				t.Errorf("completed response = %+v", event.Response)
			}
		}
	}
	if reasoningDone != "Think hard." {
		t.Errorf("reasoning done = %q", reasoningDone)
	}
	if textDeltas.String() != "Hello world" {
		t.Errorf("text deltas = %q", textDeltas.String())
	}
	if argumentDeltas.String() != `{"city":"Paris"}` || argumentsDone != `{"city":"Paris"}` {
		t.Errorf("arguments = %q / %q", argumentDeltas.String(), argumentsDone)
	}
	if customDeltas.String() != "*** Begin" || customDone != "*** Begin Patch" {
		t.Errorf("custom input = %q / %q", customDeltas.String(), customDone)
	}
	if !sawAnnotation {
		t.Error("output_text.annotation.added not decoded")
	}
	if !sawSearchAdded {
		t.Error("web search action not decoded from output_item.added")
	}
	if sawSearchStates != 3 {
		t.Errorf("web search state events = %d, want 3", sawSearchStates)
	}

	final, err := responses.Accumulate(client.CreateStream(context.Background(), &responses.Request{
		Model: "deepseek-flash",
		Input: responses.StringInput("Hi, how are you?"),
	}))
	if err != nil {
		t.Fatalf("Accumulate: %v", err)
	}
	if final.Status != responses.StatusCompleted || final.OutputText() != "Hello world" {
		t.Errorf("final response = %+v", final)
	}
	if final.Usage == nil || final.Usage.TotalTokens != 9 {
		t.Errorf("final usage = %+v", final.Usage)
	}
}

const incompleteStreamResponse = `{"id":"resp_2","object":"response","created_at":1793084910,"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"model":"deepseek-flash","output":[{"type":"message","id":"msg_2","status":"incomplete","role":"assistant","content":[{"type":"output_text","annotations":[],"logprobs":[],"text":"trunc"}],"phase":"final_answer"}],"usage":{"input_tokens":5,"output_tokens":1,"total_tokens":6}}`

func TestCreateStreamIncomplete(t *testing.T) {
	events := []sseEvent{
		eventJSON(string(responses.EventCreated), `"response":`+streamCreatedResponse, `"sequence_number":0`),
		eventJSON(string(responses.EventOutputItemAdded),
			`"item":{"type":"message","id":"msg_2","status":"in_progress","content":[],"role":"assistant"}`,
			`"output_index":0`, `"sequence_number":1`),
		eventJSON(string(responses.EventOutputTextDelta),
			`"content_index":0`, `"delta":"trunc"`, `"item_id":"msg_2"`, `"output_index":0`, `"sequence_number":2`),
		eventJSON(string(responses.EventOutputTextDone),
			`"content_index":0`, `"item_id":"msg_2"`, `"output_index":0`, `"sequence_number":3`, `"text":"trunc"`),
		eventJSON(string(responses.EventOutputItemDone),
			`"item":{"type":"message","id":"msg_2","status":"incomplete","content":[{"type":"output_text","annotations":[],"logprobs":[],"text":"trunc"}],"phase":"final_answer","role":"assistant"}`,
			`"output_index":0`, `"sequence_number":4`),
		eventJSON(string(responses.EventIncomplete),
			`"response":`+incompleteStreamResponse, `"sequence_number":5`),
	}
	client := streamingClient(t, events)

	final, err := responses.Accumulate(client.CreateStream(context.Background(), &responses.Request{
		Model: "deepseek-flash",
		Input: responses.StringInput("Write a long story."),
	}))
	if err != nil {
		t.Fatalf("Accumulate: %v", err)
	}
	if final.Status != responses.StatusIncomplete {
		t.Errorf("status = %q", final.Status)
	}
	if final.IncompleteDetails == nil || final.IncompleteDetails.Reason != responses.IncompleteMaxOutputTokens {
		t.Errorf("incomplete details = %+v", final.IncompleteDetails)
	}
	if final.OutputText() != "trunc" {
		t.Errorf("output text = %q", final.OutputText())
	}
}

func TestAccumulateWithoutTerminalEvent(t *testing.T) {
	events := []sseEvent{
		eventJSON(string(responses.EventCreated), `"response":`+streamCreatedResponse, `"sequence_number":0`),
	}
	client := streamingClient(t, events)
	_, err := responses.Accumulate(client.CreateStream(context.Background(), &responses.Request{
		Model: "deepseek-flash",
		Input: responses.StringInput("hi"),
	}))
	if err == nil {
		t.Fatal("Accumulate returned nil error")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("error = %v", err)
	}
}

func TestCreateStreamValidation(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request sent for an invalid request")
	})
	seq := client.CreateStream(context.Background(), &responses.Request{Model: "deepseek-flash"})
	count := 0
	for _, err := range seq {
		count++
		if err == nil {
			t.Fatal("expected a validation error")
		}
		var verr *deepseek.ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("error type = %T", err)
		}
	}
	if count != 1 {
		t.Errorf("yielded %d results, want 1", count)
	}
}

func TestAccumulateReturnsFailedResponse(t *testing.T) {
	failed := `{"id":"resp_1","object":"response","created_at":1,"status":"failed","model":"deepseek-flash","error":{"code":"server_error","message":"generation failed"},"output":[],"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}`
	client := streamingClient(t, []sseEvent{
		eventJSON(string(responses.EventFailed), `"response":`+failed),
	})
	resp, err := responses.Accumulate(client.CreateStream(context.Background(), &responses.Request{
		Model: "deepseek-flash",
		Input: responses.StringInput("hi"),
	}))
	var responseErr *responses.ResponseError
	if !errors.As(err, &responseErr) {
		t.Fatalf("err = %v, want *responses.ResponseError", err)
	}
	if resp == nil || resp.Usage == nil || resp.Usage.TotalTokens != 3 {
		t.Fatalf("response = %+v, want the failed response and usage", resp)
	}
}

func TestAccumulateNilStream(t *testing.T) {
	var seq iter.Seq2[responses.Event, error]
	_, err := responses.Accumulate(seq)
	var validation *deepseek.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want *deepseek.ValidationError", err)
	}
}

func TestAccumulateStopsAtTerminalEvent(t *testing.T) {
	completed := &responses.Response{ID: "resp_1", Status: responses.StatusCompleted}
	seq := func(yield func(responses.Event, error) bool) {
		if !yield(responses.Event{Type: responses.EventCompleted, Response: completed}, nil) {
			return
		}
		yield(responses.Event{}, errors.New("connection reset"))
	}
	resp, err := responses.Accumulate(seq)
	if err != nil {
		t.Fatalf("Accumulate: %v", err)
	}
	if resp != completed {
		t.Fatalf("response = %+v, want the terminal response", resp)
	}
}
