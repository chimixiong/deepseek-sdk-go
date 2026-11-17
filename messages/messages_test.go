package messages

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	deepseek "github.com/chimixiong/deepseek-sdk-go"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(deepseek.New(deepseek.Config{
		APIKey:     "secret",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}))
}

func TestCreate(t *testing.T) {
	calls := 0
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/anthropic/v1/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("x-api-key"); got != "secret" {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != APIVersion {
			t.Errorf("anthropic-version = %q", got)
		}
		if got := r.Header.Get("anthropic-beta"); got != "" {
			t.Errorf("anthropic-beta = %q, want empty", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if doc["model"] != "deepseek-flash" {
			t.Errorf("model = %#v", doc["model"])
		}
		if _, ok := doc["stream"]; ok {
			t.Errorf("stream should be omitted: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, responseFixture)
	})

	resp, err := client.Create(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if resp.ID != "msg_01" || resp.Model != "deepseek-flash" {
		t.Fatalf("response = %#v", resp)
	}
	if len(resp.Content) != 3 || resp.Content[0].Type != BlockThinking {
		t.Fatalf("content = %#v", resp.Content)
	}
}

func TestCreateRejectsStream(t *testing.T) {
	req := validRequest()
	req.Stream = true
	_, err := New(nil).Create(context.Background(), req)
	var verr *deepseek.ValidationError
	if !errors.As(err, &verr) || verr.Field != "stream" {
		t.Fatalf("error = %#v, want stream validation error", err)
	}
}

func TestCreateNilRequest(t *testing.T) {
	_, err := New(nil).Create(context.Background(), nil)
	var verr *deepseek.ValidationError
	if !errors.As(err, &verr) || verr.Field != "request" {
		t.Fatalf("error = %#v, want request validation error", err)
	}
}

func TestCreateValidationBeforeNetwork(t *testing.T) {
	_, err := New(nil).Create(context.Background(), &Request{})
	var verr *deepseek.ValidationError
	if !errors.As(err, &verr) || verr.Field != "model" {
		t.Fatalf("error = %#v, want model validation error", err)
	}
}

func TestCreateFilesBetaHeader(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("anthropic-beta"); got != FilesBeta {
			t.Errorf("anthropic-beta = %q, want %q", got, FilesBeta)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, responseFixture)
	})
	req := validRequest()
	req.Messages = []Message{{Role: RoleUser, Content: Content{Blocks: []Block{
		{Type: BlockImage, Source: &Source{Type: SourceFile, FileID: "file-api-1"}},
	}}}}
	if _, err := client.Create(context.Background(), req); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestCreateStreamValidation(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for an invalid request")
	})
	var got error
	for _, err := range client.CreateStream(context.Background(), &Request{}) {
		if err != nil {
			got = err
		}
	}
	var verr *deepseek.ValidationError
	if !errors.As(got, &verr) || verr.Field != "model" {
		t.Fatalf("error = %#v, want model validation error", got)
	}

	got = nil
	for _, err := range client.CreateStream(context.Background(), nil) {
		if err != nil {
			got = err
		}
	}
	if !errors.As(got, &verr) || verr.Field != "request" {
		t.Fatalf("error = %#v, want request validation error", got)
	}
}

func TestClientDefaults(t *testing.T) {
	client := New(nil)
	if client.api == nil {
		t.Fatal("New(nil) returned a client without a core transport")
	}
	if !strings.HasPrefix(client.api.MessagesURL(), "https://api.deepseek.com/anthropic") {
		t.Fatalf("MessagesURL = %q", client.api.MessagesURL())
	}
}
