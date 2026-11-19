package fim

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go"
)

const completionFixture = `{
  "id": "60f16544-b4f2-4149-9326-79ccc6610d67",
  "choices": [
    {
      "text": "return a + b",
      "index": 0,
      "logprobs": null,
      "finish_reason": "stop"
    }
  ],
  "created": 1793084841,
  "model": "deepseek-flash",
  "system_fingerprint": "e72d3c7f663d3e1383b00b3884cb4501",
  "object": "text_completion",
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 4,
    "total_tokens": 16,
    "prompt_tokens_details": {
      "cached_tokens": 0
    },
    "prompt_cache_hit_tokens": 0,
    "prompt_cache_miss_tokens": 12
  }
}`

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(deepseek.New(deepseek.Config{
		APIKey:      "test-key",
		BaseURL:     server.URL,
		BetaBaseURL: server.URL + "/beta",
	}))
}

func TestCreateCompletion(t *testing.T) {
	var body map[string]any
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/beta/completions" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, completionFixture)
	})

	resp, err := client.Create(context.Background(), &Request{
		Model:       "deepseek-flash",
		Prompt:      "def add(a, b):\n    ",
		Suffix:      new("\n"),
		MaxTokens:   new(32),
		Temperature: new(0.0),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if body["model"] != "deepseek-flash" || body["prompt"] != "def add(a, b):\n    " {
		t.Errorf("body = %#v", body)
	}
	if body["suffix"] != "\n" || body["max_tokens"] != float64(32) || body["temperature"] != float64(0) {
		t.Errorf("body = %#v", body)
	}
	if _, ok := body["stream"]; ok {
		t.Errorf("Create sent stream field: %#v", body)
	}
	if _, ok := body["logprobs"]; ok {
		t.Errorf("Create sent unset logprobs: %#v", body)
	}

	if resp.Object != "text_completion" || resp.Model != "deepseek-flash" || resp.Created != 1793084841 {
		t.Errorf("response = %#v", resp)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices = %d", len(resp.Choices))
	}
	choice := resp.Choices[0]
	if choice.Text != "return a + b" || choice.Index != 0 || choice.Logprobs != nil {
		t.Errorf("choice = %#v", choice)
	}
	if choice.FinishReason == nil || *choice.FinishReason != FinishStop {
		t.Errorf("finish reason = %v", choice.FinishReason)
	}
	if resp.Usage == nil || resp.Usage.PromptTokens != 12 || resp.Usage.CompletionTokens != 4 ||
		resp.Usage.TotalTokens != 16 || resp.Usage.PromptCacheMissTokens != 12 {
		t.Errorf("usage = %#v", resp.Usage)
	}
}

func TestCreateValidation(t *testing.T) {
	tests := []struct {
		name string
		req  *Request
	}{
		{name: "nil", req: nil},
		{name: "missing model", req: &Request{Prompt: "x"}},
		{name: "missing prompt", req: &Request{Model: "m"}},
		{name: "stream true", req: &Request{Model: "m", Prompt: "x", Stream: new(true)}},
		{name: "echo with suffix", req: &Request{Model: "m", Prompt: "x", Echo: new(true), Suffix: new("s")}},
		{name: "echo with logprobs", req: &Request{Model: "m", Echo: new(true), Logprobs: new(2)}},
		{name: "logprobs too high", req: &Request{Model: "m", Logprobs: new(21)}},
		{name: "stream options without stream", req: &Request{Model: "m", StreamOptions: &StreamOptions{}}},
		{name: "too many stop sequences", req: &Request{Model: "m", Stop: make(Stop, 17)}},
		{name: "temperature out of range", req: &Request{Model: "m", Temperature: new(2.1)}},
		{name: "top_p zero", req: &Request{Model: "m", TopP: new(0.0)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := New(deepseek.New(deepseek.Config{BaseURL: "http://127.0.0.1:1"}))
			if _, err := client.Create(context.Background(), tc.req); err == nil {
				t.Fatal("expected error")
			} else {
				var invalid *deepseek.ValidationError
				if !errors.As(err, &invalid) {
					t.Fatalf("error = %T %v, want *deepseek.ValidationError", err, err)
				}
			}
		})
	}
}

func TestRequestMarshal(t *testing.T) {
	one, err := json.Marshal(Request{Model: "m", Stop: Stop{"\n"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(one), `"stop":"\n"`) {
		t.Errorf("single stop = %s", one)
	}

	many, err := json.Marshal(Request{Model: "m", Stop: Stop{"\n", "END"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(many), `"stop":["\n","END"]`) {
		t.Errorf("multiple stop = %s", many)
	}

	extra, err := json.Marshal(Request{
		Model: "m",
		Extra: map[string]json.RawMessage{"custom": json.RawMessage(`{"a":1}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(extra), `"custom":{"a":1}`) {
		t.Errorf("extra = %s", extra)
	}

	empty, err := json.Marshal(Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(empty), `"stop"`) {
		t.Errorf("empty stop present = %s", empty)
	}
}

func TestCreateStream(t *testing.T) {
	const events = `data: {"id":"79323126-3e6a-4fb3-8d99-71fb50f801fc","choices":[{"text":"","index":0,"logprobs":null,"finish_reason":null}],"created":1793084906,"model":"deepseek-flash","system_fingerprint":"e72d3c7f663d3e1383b00b3884cb4501","object":"text_completion"}

data: {"id":"79323126-3e6a-4fb3-8d99-71fb50f801fc","choices":[{"text":"return","index":0,"logprobs":null,"finish_reason":null}],"created":1793084906,"model":"deepseek-flash","system_fingerprint":"e72d3c7f663d3e1383b00b3884cb4501","object":"text_completion"}

data: {"id":"79323126-3e6a-4fb3-8d99-71fb50f801fc","choices":[{"text":" a","index":0,"logprobs":null,"finish_reason":null}],"created":1793084906,"model":"deepseek-flash","system_fingerprint":"e72d3c7f663d3e1383b00b3884cb4501","object":"text_completion"}

data: {"id":"79323126-3e6a-4fb3-8d99-71fb50f801fc","choices":[{"text":" + b","index":0,"logprobs":null,"finish_reason":"stop"}],"created":1793084906,"model":"deepseek-flash","system_fingerprint":"e72d3c7f663d3e1383b00b3884cb4501","object":"text_completion","usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16,"prompt_tokens_details":{"cached_tokens":0},"prompt_cache_hit_tokens":0,"prompt_cache_miss_tokens":12}}

data: [DONE]

`

	var body map[string]any
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/beta/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, events)
	})

	stream, err := client.CreateStream(context.Background(), &Request{
		Model:  "deepseek-flash",
		Prompt: "def add(a, b):\n    ",
	})
	if err != nil {
		t.Fatalf("CreateStream: %v", err)
	}
	defer stream.Close()

	if body["stream"] != true {
		t.Errorf("stream = %#v", body["stream"])
	}
	options, _ := body["stream_options"].(map[string]any)
	if options["include_usage"] != true {
		t.Errorf("stream_options = %#v", body["stream_options"])
	}

	var text strings.Builder
	chunks := 0
	var usage *Usage
	for {
		chunk, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		chunks++
		if len(chunk.Choices) > 0 {
			text.WriteString(chunk.Choices[0].Text)
			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason != FinishStop {
				t.Errorf("finish reason = %q", *chunk.Choices[0].FinishReason)
			}
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}
	if chunks != 4 {
		t.Errorf("chunks = %d, want 4", chunks)
	}
	if got := text.String(); got != "return a + b" {
		t.Errorf("text = %q", got)
	}
	if usage == nil || usage.TotalTokens != 16 {
		t.Errorf("usage = %#v", usage)
	}
}

func TestStopUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Stop
	}{
		{"string", `"END"`, Stop{"END"}},
		{"array", `["END","\n"]`, Stop{"END", "\n"}},
		{"null", `null`, nil},
		{"empty array", `[]`, Stop{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var r Request
			raw := `{"model":"m","prompt":"p","stop":` + tc.in + `}`
			if err := json.Unmarshal([]byte(raw), &r); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if !slices.Equal(r.Stop, tc.want) {
				t.Fatalf("stop = %#v, want %#v", r.Stop, tc.want)
			}
		})
	}
}

func TestRequestValidateNilReceiver(t *testing.T) {
	var req *Request
	err := req.Validate()
	var invalid *deepseek.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %T %v, want *deepseek.ValidationError", err, err)
	}
	if invalid.Field != "request" {
		t.Fatalf("field = %q, want request", invalid.Field)
	}
}

func TestNewNilClient(t *testing.T) {
	if client := New(nil); client == nil || client.api == nil {
		t.Fatal("New(nil) did not install a default transport client")
	}
}

func TestStreamTruncatedBeforeDone(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"text\":\"half\",\"index\":0}]}\n\n")
	})
	stream, err := client.CreateStream(context.Background(), &Request{Model: "deepseek-flash", Prompt: "x"})
	if err != nil {
		t.Fatalf("CreateStream: %v", err)
	}
	defer stream.Close()
	if _, err := stream.Next(); err != nil {
		t.Fatalf("first Next: %v", err)
	}
	_, err = stream.Next()
	if err == nil || errors.Is(err, io.EOF) || !strings.Contains(err.Error(), "before [DONE]") {
		t.Fatalf("Next after truncation = %v, want a non-EOF error", err)
	}
}

func TestNilClientDoesNotPanic(t *testing.T) {
	var client *Client
	if _, err := client.Create(context.Background(), &Request{Model: "m", Prompt: "p"}); err == nil {
		t.Fatal("Create on a nil client returned no error")
	}
	if _, err := client.CreateStream(context.Background(), &Request{Model: "m", Prompt: "p"}); err == nil {
		t.Fatal("CreateStream on a nil client returned no error")
	}
}
