package chat_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go"
	"github.com/chimixiong/deepseek-sdk-go/chat"
)

func newClient(t *testing.T, handler http.HandlerFunc) (*chat.Client, *deepseek.Client) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	core := deepseek.New(deepseek.Config{
		APIKey:         "test-key",
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
		MaxRetries:     -1,
		RetryBaseDelay: 0,
	})
	return chat.New(core), core
}

func TestCreateRequestAndResponse(t *testing.T) {
	var captured map[string]json.RawMessage
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{
			"id":"chat-1","object":"chat.completion","created":1,"model":"deepseek-flash",
			"system_fingerprint":"fp",
			"choices":[{
				"index":0,
				"message":{
					"role":"assistant",
					"content":"hello",
					"reasoning_content":"thinking",
					"tool_calls":[{"id":"call_2","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Hangzhou\"}"}}],
					"refusal":null
				},
				"finish_reason":"tool_calls",
				"logprobs":{"content":[{"token":"hello","logprob":-0.1,"bytes":[104],"top_logprobs":[{"token":"hello","logprob":-0.1}]}]}
			}],
			"usage":{
				"prompt_tokens":7,"completion_tokens":3,"total_tokens":10,
				"prompt_tokens_details":{"cached_tokens":2},
				"prompt_cache_hit_tokens":2,"prompt_cache_miss_tokens":5,
				"completion_tokens_details":{"reasoning_tokens":1},
				"server_tool_use":{"tool_search":1}
			}
		}`)
	})

	req := &chat.Request{
		Model: "deepseek-flash",
		Messages: []chat.Message{
			chat.SystemMessage("be brief"),
			chat.UserContentMessage(chat.TextPart("what is this?"), chat.ImagePart("https://example.com/a.jpg", "low")),
			chat.AssistantToolCalls("need weather", chat.ToolCall{
				ID: "call_1", Type: "function",
				Function: chat.FunctionCall{Name: "get_weather", Arguments: `{"city":"Hangzhou"}`},
			}),
			chat.ToolMessage("call_1", chat.TextContent("sunny")),
		},
		Thinking:        &chat.Thinking{Type: chat.ThinkingDisabled},
		ReasoningEffort: chat.EffortLow,
		MaxTokens:       new(256),
		ResponseFormat:  &chat.ResponseFormat{Type: chat.FormatJSONObject},
		Stop:            chat.Stop{"END"},
		Tools:           []chat.Tool{chat.FunctionTool("get_weather", "weather", map[string]any{"type": "object"})},
		ToolChoice:      chat.AutoToolChoice(),
		SafetyCheck:     &chat.SafetyCheck{Level: chat.SafetyLow},
		Title:           &chat.Title{Enabled: true},
		Temperature:     new(0.2),
		TopP:            new(0.9),
		Logprobs:        new(true),
		TopLogprobs:     new(2),
		UserID:          "u1",
	}
	response, err := client.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got := string(captured["stream"]); got != "false" {
		t.Errorf("stream = %s", got)
	}
	if _, ok := captured["stream_options"]; ok {
		t.Error("stream_options must be absent for non-streaming requests")
	}
	if got := string(captured["stop"]); got != `"END"` {
		t.Errorf("stop = %s", got)
	}
	if got := string(captured["tool_choice"]); got != `"auto"` {
		t.Errorf("tool_choice = %s", got)
	}
	var messages []chat.Message
	if err := json.Unmarshal(captured["messages"], &messages); err != nil {
		t.Fatalf("decode messages: %v", err)
	}
	if messages[1].Content.Parts[1].ImageURL.URL != "https://example.com/a.jpg" {
		t.Errorf("image part = %+v", messages[1].Content.Parts[1])
	}
	if messages[2].ReasoningContent == nil || *messages[2].ReasoningContent != "need weather" {
		t.Errorf("assistant reasoning = %v", messages[2].ReasoningContent)
	}
	if messages[3].ToolCallID != "call_1" || messages[3].Content.Text != "sunny" {
		t.Errorf("tool message = %+v", messages[3])
	}

	message := response.AssistantMessage()
	if message.Content.Text != "hello" || response.ReasoningContent() != "thinking" {
		t.Errorf("message = %+v", message)
	}
	if len(message.ToolCalls) != 1 || message.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("tool calls = %+v", message.ToolCalls)
	}
	if response.Choices[0].FinishReason != chat.FinishToolCalls {
		t.Errorf("finish = %q", response.Choices[0].FinishReason)
	}
	if response.Usage == nil || response.Usage.PromptTokensDetails.CachedTokens != 2 ||
		response.Usage.CompletionDetails == nil || response.Usage.CompletionDetails.ReasoningTokens != 1 {
		t.Errorf("usage = %+v", response.Usage)
	}
	if got := response.Choices[0].Logprobs.Content[0].TopLogprobs[0].Token; got != "hello" {
		t.Errorf("logprobs = %+v", response.Choices[0].Logprobs)
	}
}

func TestCreateError(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ds-trace-id", "trace-1")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"message":"invalid model","type":"invalid_request_error","code":"invalid_model"}}`)
	})
	_, err := client.Create(context.Background(), &chat.Request{
		Model:    "deepseek-flash",
		Messages: []chat.Message{chat.UserMessage("hi")},
	})
	var apiErr *deepseek.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v", err)
	}
	if apiErr.StatusCode != 400 || apiErr.RequestID != "trace-1" || apiErr.Code != "invalid_model" {
		t.Fatalf("apiErr = %+v", apiErr)
	}
}

func TestBetaPrefix(t *testing.T) {
	var path string
	client, core := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		io.WriteString(w, `{"id":"1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"x"},"finish_reason":"stop"}]}`)
	})

	prefix := true
	req := &chat.Request{
		Model: "deepseek-flash",
		Messages: []chat.Message{{
			Role:    chat.RoleAssistant,
			Content: chat.TextContent("Once upon"),
			Prefix:  &prefix,
		}},
	}
	if _, err := client.Create(context.Background(), req); err == nil || !strings.Contains(err.Error(), "beta") {
		t.Fatalf("standard client err = %v, want beta error", err)
	}

	beta := chat.NewBeta(core)
	if _, err := beta.Create(context.Background(), req); err != nil {
		t.Fatalf("beta Create: %v", err)
	}
	if path != "/beta/chat/completions" {
		t.Fatalf("path = %q", path)
	}
}

func TestNewNilClient(t *testing.T) {
	if chat.New(nil) == nil {
		t.Fatal("chat.New(nil) returned nil")
	}
	if chat.NewBeta(nil) == nil {
		t.Fatal("chat.NewBeta(nil) returned nil")
	}
}
