package chat_test

import (
	"context"
	"encoding/json"
	"io"
	"iter"
	"net/http"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go/chat"
)

const streamFixture = `data: {"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","system_fingerprint":"fp","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"think"}}]}

: keep-alive

data: {"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"reasoning_content":"ing"}}]}

data: {"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"content":"Hel"}}]}

data: {"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"content":"lo"}}]}

data: {"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"cit"}}]}}]}

data: {"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"y\":\"Hangzhou\"}"}}]}}]}

data: {"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":5,"completion_tokens":4,"total_tokens":9,"prompt_tokens_details":{"cached_tokens":0},"prompt_cache_hit_tokens":0,"prompt_cache_miss_tokens":5}}

data: [DONE]

`

func TestCreateStreamAndAccumulate(t *testing.T) {
	var request map[string]json.RawMessage
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, streamFixture)
	})

	req := &chat.Request{
		Model:    "deepseek-flash",
		Messages: []chat.Message{chat.UserMessage("weather?")},
		Tools:    []chat.Tool{chat.FunctionTool("get_weather", "weather", nil)},
	}
	response, err := chat.Accumulate(client.CreateStream(context.Background(), req))
	if err != nil {
		t.Fatalf("Accumulate: %v", err)
	}

	if string(request["stream"]) != "true" {
		t.Errorf("stream = %s", request["stream"])
	}
	var options chat.StreamOptions
	if err := json.Unmarshal(request["stream_options"], &options); err != nil {
		t.Fatalf("stream_options: %v", err)
	}
	if options.IncludeUsage == nil || !*options.IncludeUsage {
		t.Errorf("include_usage = %+v", options.IncludeUsage)
	}

	message := response.AssistantMessage()
	if message.Content.Text != "Hello" {
		t.Errorf("content = %q", message.Content.Text)
	}
	if response.ReasoningContent() != "thinking" {
		t.Errorf("reasoning = %q", response.ReasoningContent())
	}
	if len(message.ToolCalls) != 1 {
		t.Fatalf("tool calls = %+v", message.ToolCalls)
	}
	call := message.ToolCalls[0]
	if call.ID != "call_1" || call.Function.Name != "get_weather" || call.Function.Arguments != `{"city":"Hangzhou"}` {
		t.Errorf("call = %+v", call)
	}
	if response.Choices[0].FinishReason != chat.FinishToolCalls {
		t.Errorf("finish = %q", response.Choices[0].FinishReason)
	}
	if response.Usage == nil || response.Usage.TotalTokens != 9 {
		t.Errorf("usage = %+v", response.Usage)
	}
}

func TestCreateStreamSkipsEmptyDataFrames(t *testing.T) {
	fixture := "data:\n\n" +
		"data: \n\n" +
		"event: ping\n\n" +
		`data: {"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"content":"hi"}}]}` + "\n\n" +
		"data: [DONE]\n\n"
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, fixture)
	})
	response, err := chat.Accumulate(client.CreateStream(context.Background(), &chat.Request{
		Model:    "deepseek-flash",
		Messages: []chat.Message{chat.UserMessage("hi")},
	}))
	if err != nil {
		t.Fatalf("Accumulate: %v", err)
	}
	if got := response.Choices[0].Message.Content.Text; got != "hi" {
		t.Fatalf("content = %q, want %q", got, "hi")
	}
}

func TestCreateStreamDecodeError(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "data: {not json}\n\n")
	})
	_, err := chat.Accumulate(client.CreateStream(context.Background(), &chat.Request{
		Model:    "deepseek-flash",
		Messages: []chat.Message{chat.UserMessage("hi")},
	}))
	if err == nil {
		t.Fatal("expected a decode error")
	}
}

func chunkSeq(chunks ...chat.Chunk) iter.Seq2[chat.Chunk, error] {
	return func(yield func(chat.Chunk, error) bool) {
		for _, chunk := range chunks {
			if !yield(chunk, nil) {
				return
			}
		}
	}
}

func TestAccumulateAppendsFragments(t *testing.T) {
	chunks := []chat.Chunk{
		{ID: "chat-1", Choices: []chat.ChunkChoice{{
			Index: 0,
			Delta: chat.Delta{
				Refusal:         "I can",
				Title:           "A ",
				Annotations:     []json.RawMessage{json.RawMessage(`{"a":1}`)},
				ServerToolCalls: []json.RawMessage{json.RawMessage(`{"s":1}`)},
			},
		}}},
		{ID: "chat-1", Choices: []chat.ChunkChoice{{
			Index: 0,
			Delta: chat.Delta{
				Refusal:         " not help",
				Title:           "title",
				Annotations:     []json.RawMessage{json.RawMessage(`{"a":2}`)},
				ServerToolCalls: []json.RawMessage{json.RawMessage(`{"s":2}`)},
			},
		}}},
	}
	response, err := chat.Accumulate(chunkSeq(chunks...))
	if err != nil {
		t.Fatalf("Accumulate: %v", err)
	}
	message := response.AssistantMessage()
	if message.Refusal != "I can not help" {
		t.Errorf("refusal = %q", message.Refusal)
	}
	if message.Title != "A title" {
		t.Errorf("title = %q", message.Title)
	}
	if len(message.Annotations) != 2 || string(message.Annotations[1]) != `{"a":2}` {
		t.Errorf("annotations = %v", message.Annotations)
	}
	if len(message.ServerToolCalls) != 2 || string(message.ServerToolCalls[0]) != `{"s":1}` {
		t.Errorf("server tool calls = %v", message.ServerToolCalls)
	}
}

func TestAccumulateMergesLogprobs(t *testing.T) {
	chunks := []chat.Chunk{
		{Choices: []chat.ChunkChoice{{
			Delta: chat.Delta{Content: new("He")},
			Logprobs: &chat.ChoiceLogprobs{
				Content:          []chat.TokenLogprob{{Token: "He", Logprob: -0.1}},
				ReasoningContent: []chat.TokenLogprob{{Token: "r1", Logprob: -0.2}},
			},
		}}},
		{Choices: []chat.ChunkChoice{{
			Delta: chat.Delta{Content: new("llo")},
			Logprobs: &chat.ChoiceLogprobs{
				Content:          []chat.TokenLogprob{{Token: "llo", Logprob: -0.3}},
				ReasoningContent: []chat.TokenLogprob{{Token: "r2", Logprob: -0.4}},
			},
		}}},
	}
	response, err := chat.Accumulate(chunkSeq(chunks...))
	if err != nil {
		t.Fatalf("Accumulate: %v", err)
	}
	logprobs := response.Choices[0].Logprobs
	if logprobs == nil {
		t.Fatal("logprobs must not be nil")
	}
	if len(logprobs.Content) != 2 || logprobs.Content[0].Token != "He" || logprobs.Content[1].Logprob != -0.3 {
		t.Errorf("content logprobs = %+v", logprobs.Content)
	}
	if len(logprobs.ReasoningContent) != 2 || logprobs.ReasoningContent[1].Token != "r2" {
		t.Errorf("reasoning logprobs = %+v", logprobs.ReasoningContent)
	}
}

func TestAccumulateLeavesLogprobsNil(t *testing.T) {
	response, err := chat.Accumulate(chunkSeq(chat.Chunk{Choices: []chat.ChunkChoice{{Delta: chat.Delta{Content: new("hi")}}}}))
	if err != nil {
		t.Fatalf("Accumulate: %v", err)
	}
	if response.Choices[0].Logprobs != nil {
		t.Errorf("logprobs = %+v, want nil", response.Choices[0].Logprobs)
	}
}

func TestAccumulateNilStream(t *testing.T) {
	response, err := chat.Accumulate(nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if response != nil {
		t.Fatalf("response = %+v", response)
	}
}
