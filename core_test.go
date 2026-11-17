package deepseek_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chimixiong/deepseek-sdk-go"
)

func newTestClient(t *testing.T, server *httptest.Server) *deepseek.Client {
	t.Helper()
	return deepseek.New(deepseek.Config{
		APIKey:         "test-key",
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
		MaxRetries:     1,
		RetryBaseDelay: 0,
	})
}

func TestDoJSONRequestShape(t *testing.T) {
	var gotPath, gotQuery, gotAuth, gotAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotAgent = r.Header.Get("User-Agent")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["model"] != "deepseek-flash" {
			t.Errorf("model = %v", body["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	var out struct {
		OK bool `json:"ok"`
	}
	err := client.DoJSON(context.Background(), &deepseek.Request{
		Method: http.MethodGet,
		URL:    client.BaseURL() + "/models",
		Query:  url.Values{"limit": []string{"5"}, "order": []string{"desc"}},
		JSON:   map[string]any{"model": "deepseek-flash"},
	}, &out)
	if err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if !out.OK {
		t.Fatal("response not decoded")
	}
	if gotPath != "/models" {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery != "limit=5&order=desc" {
		t.Errorf("query = %q", gotQuery)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("authorization = %q", gotAuth)
	}
	wantAgent := "deepseek-sdk-go/" + deepseek.Version + " (+https://github.com/chimixiong/deepseek-sdk-go)"
	if gotAgent != wantAgent {
		t.Errorf("user agent = %q, want %q", gotAgent, wantAgent)
	}
}

func TestMultipartPartOrder(t *testing.T) {
	var order []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			t.Errorf("content type = %q (%v)", mediaType, err)
			return
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("next part: %v", err)
				return
			}
			order = append(order, part.FormName())
		}
		io.WriteString(w, `{}`)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	err := client.DoJSON(context.Background(), &deepseek.Request{
		URL: client.FilesURL(),
		Multipart: []deepseek.MultipartPart{
			{Field: "purpose", Value: []byte("user_data")},
			{Field: "file", Filename: "image.jpg", ContentType: "image/jpeg", Value: []byte{0xff, 0xd8}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if len(order) != 2 || order[0] != "purpose" || order[1] != "file" {
		t.Fatalf("part order = %v", order)
	}
}

func TestSSEParsing(t *testing.T) {
	body := strings.Join([]string{
		": keep-alive",
		"event: response.created",
		"data: {\"type\":\"response.created\"}",
		"",
		"data: first",
		"data: second",
		"",
		"data: [DONE]",
		"",
		"",
	}, "\n")
	stream := deepseek.NewStream(io.NopCloser(strings.NewReader(body)))

	event, err := stream.Next()
	if err != nil || event.Event != "response.created" || string(event.Data) != `{"type":"response.created"}` {
		t.Fatalf("event 1 = %+v, %v", event, err)
	}
	event, err = stream.Next()
	if err != nil || string(event.Data) != "first\nsecond" {
		t.Fatalf("event 2 = %+v, %v", event, err)
	}
	event, err = stream.Next()
	if err != nil || string(event.Data) != "[DONE]" {
		t.Fatalf("event 3 = %+v, %v", event, err)
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("final err = %v, want EOF", err)
	}
}

func TestRetryThenSuccess(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"error":{"message":"slow down","type":"rate_limit","code":"rate_limit_exceeded"}}`)
			return
		}
		io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	err := client.DoJSON(context.Background(), &deepseek.Request{URL: server.URL + "/x", JSON: map[string]any{}}, nil)
	if err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestNoRetryOnBadRequest(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("x-request-id", "req_123")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"message":"bad model","type":"invalid_request_error","code":"invalid_model","param":"model"}}`)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	err := client.DoJSON(context.Background(), &deepseek.Request{URL: server.URL + "/x", JSON: map[string]any{}}, nil)
	var apiErr *deepseek.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.Message != "bad model" || apiErr.Code != "invalid_model" || apiErr.Param != "model" || apiErr.RequestID != "req_123" {
		t.Fatalf("apiErr = %+v", apiErr)
	}
}

func TestDoRawBinary(t *testing.T) {
	payload := []byte{0x52, 0x49, 0x46, 0x46}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "audio/wav" {
			t.Errorf("accept = %q", got)
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(payload)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	resp, err := client.Do(context.Background(), &deepseek.Request{
		URL:    server.URL + "/v1/tools/tts",
		JSON:   map[string]any{"text": "hi"},
		Header: http.Header{"Accept": []string{"audio/wav"}},
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload = %v", got)
	}
	if resp.Header.Get("Content-Type") != "audio/wav" {
		t.Fatalf("content type = %q", resp.Header.Get("Content-Type"))
	}
}

func TestMarshalWithExtra(t *testing.T) {
	body, err := deepseek.MarshalWithExtra(map[string]any{"a": 1}, map[string]json.RawMessage{
		"b": json.RawMessage(`{"c":2}`),
	})
	if err != nil {
		t.Fatalf("MarshalWithExtra: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(fields["a"]) != "1" || string(fields["b"]) != `{"c":2}` {
		t.Fatalf("fields = %v", fields)
	}
}

func TestMultipartEscapesHeaderInjection(t *testing.T) {
	type capture struct {
		header   textproto.MIMEHeader
		filename string
		parts    int
	}
	captured := make(chan capture, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c capture
		defer func() { captured <- c }()
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Errorf("content type: %v", err)
			return
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("next part: %v", err)
				return
			}
			c.parts++
			c.header = part.Header
			c.filename = part.FileName()
		}
		io.WriteString(w, `{}`)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	err := client.DoJSON(context.Background(), &deepseek.Request{
		URL: client.FilesURL(),
		Multipart: []deepseek.MultipartPart{{
			Field:    "file",
			Filename: "ok\r\nX-Injected: yes\r\n\r\nignored",
			Value:    []byte{0x01},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	got := <-captured
	if got.parts != 1 {
		t.Fatalf("parts = %d, want 1", got.parts)
	}
	if injected := got.header.Get("X-Injected"); injected != "" {
		t.Fatalf("injected header present: %q", injected)
	}
	want := "ok%0D%0AX-Injected: yes%0D%0A%0D%0Aignored"
	if got.filename != want {
		t.Fatalf("filename = %q, want %q", got.filename, want)
	}
}

func TestHeaderOverridesDefaults(t *testing.T) {
	captured := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured <- r.Header.Clone()
		io.WriteString(w, `{}`)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	err := client.DoJSON(context.Background(), &deepseek.Request{
		URL:  server.URL + "/x",
		JSON: map[string]any{"a": 1},
		Header: http.Header{
			"Content-Type":  {"application/json; charset=utf-8"},
			"Authorization": {"Bearer custom"},
			"X-Trace":       {"a", "b"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	got := <-captured
	if values := got.Values("Content-Type"); len(values) != 1 || values[0] != "application/json; charset=utf-8" {
		t.Fatalf("content type = %v", values)
	}
	if values := got.Values("Authorization"); len(values) != 1 || values[0] != "Bearer custom" {
		t.Fatalf("authorization = %v", values)
	}
	if values := got.Values("X-Trace"); len(values) != 2 {
		t.Fatalf("x-trace = %v", values)
	}
}

func TestSSESkipsEventsWithoutData(t *testing.T) {
	stream := deepseek.NewStream(io.NopCloser(strings.NewReader("event: ping\n\nid: 7\n\n: keep-alive\n\ndata: real\n\n")))
	event, err := stream.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if event.Event != "" || event.ID != "" || string(event.Data) != "real" {
		t.Fatalf("event = %+v", event)
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("final err = %v, want EOF", err)
	}
}

func TestSSESkipsEmptyDataPayload(t *testing.T) {
	// A data field with no value, and one holding only a space, both leave an
	// empty payload and must not be dispatched (SSE spec).
	stream := deepseek.NewStream(io.NopCloser(strings.NewReader("data:\n\ndata: \n\ndata:\r\n\r\ndata: real\n\n")))
	event, err := stream.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if string(event.Data) != "real" {
		t.Fatalf("data = %q, want %q", event.Data, "real")
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("final err = %v, want EOF", err)
	}
}

func TestSSELineEndings(t *testing.T) {
	// CRLF, a bare CR and a final line without any terminator.
	stream := deepseek.NewStream(io.NopCloser(strings.NewReader("data: a\r\n\r\ndata: b\r\rdata: c\r")))
	for _, want := range []string{"a", "b", "c"} {
		event, err := stream.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if string(event.Data) != want {
			t.Fatalf("data = %q, want %q", event.Data, want)
		}
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("final err = %v, want EOF", err)
	}
}

func TestNilStream(t *testing.T) {
	stream := deepseek.NewStream(nil)
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want EOF", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestDoJSONEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	var out struct {
		OK bool `json:"ok"`
	}
	if err := client.DoJSON(context.Background(), &deepseek.Request{URL: server.URL + "/x"}, &out); err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
}

func TestNoRetryAfterRequestBodyWritten(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		io.Copy(io.Discard, r.Body)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		conn.Close()
	}))
	defer server.Close()

	client := deepseek.New(deepseek.Config{
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
		MaxRetries:     3,
		RetryBaseDelay: -1,
	})
	err := client.DoJSON(context.Background(), &deepseek.Request{URL: server.URL + "/x", JSON: map[string]any{"a": 1}}, nil)
	var transportErr *deepseek.TransportError
	if !errors.As(err, &transportErr) {
		t.Fatalf("err = %v, want *TransportError", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1: a POST that reached the server must not be replayed", got)
	}
}

func TestNegativeRetryBaseDelaySkipsWait(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"slow down"}}`)
	}))
	defer server.Close()

	client := deepseek.New(deepseek.Config{
		BaseURL:        server.URL,
		HTTPClient:     server.Client(),
		MaxRetries:     3,
		RetryBaseDelay: -1,
	})
	start := time.Now()
	err := client.DoJSON(context.Background(), &deepseek.Request{URL: server.URL + "/x"}, nil)
	if err == nil {
		t.Fatal("want error")
	}
	if got := attempts.Load(); got != 4 {
		t.Fatalf("attempts = %d, want 4", got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("elapsed = %v, want no backoff wait", elapsed)
	}
}
