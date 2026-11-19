package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go"
)

const fileFixture = `{
  "id": "file-api-0a1b2c3d4e5f60718293a4b5c6d7e8f9",
  "object": "file",
  "bytes": 102400,
  "created_at": 1700000000,
  "filename": "image.jpg",
  "purpose": "user_data"
}`

const listFixture = `{
  "object": "list",
  "data": [
    {
      "id": "file-api-0a1b2c3d4e5f60718293a4b5c6d7e8f9",
      "object": "file",
      "bytes": 102400,
      "created_at": 1700000000,
      "filename": "image.jpg",
      "purpose": "user_data"
    }
  ],
  "first_id": "file-api-0a1b2c3d4e5f60718293a4b5c6d7e8f9",
  "last_id": "file-api-0a1b2c3d4e5f60718293a4b5c6d7e8f9",
  "has_more": false
}`

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(deepseek.New(deepseek.Config{APIKey: "test-key", BaseURL: server.URL}))
}

func TestCreate(t *testing.T) {
	const response = `{
  "id": "file-api-0a1b2c3d4e5f60718293a4b5c6d7e8f9",
  "object": "file",
  "bytes": 4,
  "created_at": 1700000000,
  "filename": "image.jpg",
  "purpose": "user_data",
  "expires_at": 1700086400
}`
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/files" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			t.Errorf("content type = %q, err = %v", r.Header.Get("Content-Type"), err)
			return
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		reader := multipart.NewReader(bytes.NewReader(raw), params["boundary"])
		var names, purposes []string
		var fileContent string
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Errorf("next part: %v", err)
				return
			}
			value, _ := io.ReadAll(part)
			names = append(names, part.FormName())
			switch part.FormName() {
			case "purpose":
				purposes = append(purposes, string(value))
			case "file":
				if part.FileName() != "image.jpg" {
					t.Errorf("filename = %q", part.FileName())
				}
				fileContent = string(value)
			}
		}
		want := []string{"purpose", "file", "expires_after[anchor]", "expires_after[seconds]"}
		if strings.Join(names, ",") != strings.Join(want, ",") {
			t.Errorf("part order = %v, want %v", names, want)
		}
		if len(purposes) != 1 || purposes[0] != "user_data" {
			t.Errorf("purpose = %v", purposes)
		}
		if fileContent != "data" {
			t.Errorf("file content = %q", fileContent)
		}
		if purposeAt, fileAt := strings.Index(string(raw), `name="purpose"`), strings.Index(string(raw), `name="file"`); purposeAt < 0 || fileAt < 0 || purposeAt > fileAt {
			t.Errorf("purpose must precede file in raw body")
		}
		if anchorAt, secondsAt := strings.Index(string(raw), `name="expires_after[anchor]"`), strings.Index(string(raw), `name="expires_after[seconds]"`); anchorAt < 0 || secondsAt < 0 || anchorAt > secondsAt {
			t.Errorf("expires_after anchor must precede seconds")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, response)
	})

	file, err := client.Create(context.Background(), &CreateRequest{
		Purpose:      PurposeUserData,
		File:         []byte("data"),
		Filename:     "image.jpg",
		ContentType:  "image/jpeg",
		ExpiresAfter: &ExpiresAfter{Anchor: AnchorCreatedAt, Seconds: 86400},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if file.ID != "file-api-0a1b2c3d4e5f60718293a4b5c6d7e8f9" || file.Object != "file" {
		t.Errorf("file = %#v", file)
	}
	if file.Bytes != 4 || file.CreatedAt != 1700000000 || file.Filename != "image.jpg" || file.Purpose != PurposeUserData {
		t.Errorf("file = %#v", file)
	}
	if file.ExpiresAt == nil || *file.ExpiresAt != 1700086400 {
		t.Errorf("expires_at = %v", file.ExpiresAt)
	}
}

func TestCreateExpiresAnchorDefaults(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "created_at") {
			t.Errorf("body lacks default anchor: %s", raw)
		}
		if !strings.Contains(string(raw), "3600") {
			t.Errorf("body lacks seconds: %s", raw)
		}
		io.WriteString(w, fileFixture)
	})
	if _, err := client.Create(context.Background(), &CreateRequest{
		Purpose:      PurposeUserData,
		File:         []byte("x"),
		Filename:     "x.png",
		ExpiresAfter: &ExpiresAfter{Seconds: 3600},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestCreateWithoutExpiry(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), "expires_after") {
			t.Errorf("body contains expiry: %s", raw)
		}
		io.WriteString(w, fileFixture)
	})
	if _, err := client.Create(context.Background(), &CreateRequest{
		Purpose:  PurposeUserData,
		File:     []byte("x"),
		Filename: "x.png",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestCreateValidation(t *testing.T) {
	tests := []struct {
		name string
		req  *CreateRequest
	}{
		{name: "nil", req: nil},
		{name: "missing purpose", req: &CreateRequest{File: []byte("x"), Filename: "x"}},
		{name: "missing filename", req: &CreateRequest{Purpose: PurposeUserData, File: []byte("x")}},
		{name: "unknown purpose", req: &CreateRequest{Purpose: "vision", Filename: "x", File: []byte("x")}},
		{name: "empty content", req: &CreateRequest{Purpose: PurposeUserData, Filename: "x"}},
		{name: "bad anchor", req: &CreateRequest{Purpose: PurposeUserData, Filename: "x", File: []byte("x"), ExpiresAfter: &ExpiresAfter{Anchor: "now", Seconds: 3600}}},
		{name: "seconds too low", req: &CreateRequest{Purpose: PurposeUserData, Filename: "x", File: []byte("x"), ExpiresAfter: &ExpiresAfter{Seconds: 3599}}},
		{name: "seconds too high", req: &CreateRequest{Purpose: PurposeUserData, Filename: "x", File: []byte("x"), ExpiresAfter: &ExpiresAfter{Seconds: 2592001}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := New(deepseek.New(deepseek.Config{BaseURL: "http://127.0.0.1:1"}))
			_, err := client.Create(context.Background(), tc.req)
			if err == nil {
				t.Fatal("expected error")
			}
			var invalid *deepseek.ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("error = %T %v, want *deepseek.ValidationError", err, err)
			}
		})
	}
}

func TestList(t *testing.T) {
	var query url.Values
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/files" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		query = r.URL.Query()
		io.WriteString(w, listFixture)
	})
	list, err := client.List(context.Background(), &ListOptions{
		After:   "file-api-cursor",
		Limit:   new(2),
		Order:   OrderDesc,
		Purpose: PurposeTextRecognition,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := map[string]string{
		"after":   "file-api-cursor",
		"limit":   "2",
		"order":   "desc",
		"purpose": "text_recognition",
	}
	for key, value := range want {
		if got := query.Get(key); got != value {
			t.Errorf("query %s = %q, want %q", key, got, value)
		}
	}
	if list.Object != "list" || len(list.Data) != 1 || list.HasMore {
		t.Errorf("list = %#v", list)
	}
	if list.FirstID != "file-api-0a1b2c3d4e5f60718293a4b5c6d7e8f9" || list.LastID != list.FirstID {
		t.Errorf("cursors = %q %q", list.FirstID, list.LastID)
	}
	file := list.Data[0]
	if file.Bytes != 102400 || file.CreatedAt != 1700000000 || file.Purpose != PurposeUserData {
		t.Errorf("file = %#v", file)
	}
}

func TestListDefaultsAndValidation(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		io.WriteString(w, listFixture)
	})
	if _, err := client.List(context.Background(), nil); err != nil {
		t.Fatalf("List(nil): %v", err)
	}

	for _, options := range []*ListOptions{
		{Limit: new(0)},
		{Limit: new(1001)},
		{Order: "sideways"},
	} {
		if _, err := client.List(context.Background(), options); err == nil {
			t.Errorf("List(%+v) expected error", options)
		} else {
			var invalid *deepseek.ValidationError
			if !errors.As(err, &invalid) {
				t.Errorf("error = %T %v, want *deepseek.ValidationError", err, err)
			}
		}
	}
}

func TestRetrieve(t *testing.T) {
	const id = "file-api-1/2"
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		if got := r.URL.EscapedPath(); got != "/files/file-api-1%2F2" {
			t.Errorf("path = %q", got)
		}
		io.WriteString(w, fileFixture)
	})
	file, err := client.Retrieve(context.Background(), id)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if file.ID != "file-api-0a1b2c3d4e5f60718293a4b5c6d7e8f9" {
		t.Errorf("file = %#v", file)
	}
	if _, err := client.Retrieve(context.Background(), ""); err == nil {
		t.Error("expected error for empty id")
	}
}

func TestDelete(t *testing.T) {
	const fixture = `{"id":"file-api-1","object":"file","deleted":true}`
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/files/file-api-1" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, fixture)
	})
	deleted, err := client.Delete(context.Background(), "file-api-1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !deleted.Deleted || deleted.ID != "file-api-1" || deleted.Object != "file" {
		t.Errorf("deleted = %#v", deleted)
	}
	if _, err := client.Delete(context.Background(), ""); err == nil {
		t.Error("expected error for empty id")
	}
}

func TestNewNilClient(t *testing.T) {
	if client := New(nil); client == nil || client.api == nil {
		t.Fatal("New(nil) did not install a default transport client")
	}
}

func TestNilClientMethods(t *testing.T) {
	var client *Client
	if _, err := client.List(context.Background(), nil); err == nil {
		t.Fatal("List on a nil client returned no error")
	}
	if _, err := client.Retrieve(context.Background(), "file-1"); err == nil {
		t.Fatal("Retrieve on a nil client returned no error")
	}
	if _, err := client.Delete(context.Background(), "file-1"); err == nil {
		t.Fatal("Delete on a nil client returned no error")
	}
}

func TestFileSizeLimit(t *testing.T) {
	tests := []struct {
		contentType string
		filename    string
		want        int
	}{
		{"image/png", "a.png", maxImageFileBytes},
		{"application/pdf", "a.pdf", maxDocumentFileBytes},
		{"", "deck.pptx", maxDocumentFileBytes},
		{"text/plain", "a.txt", maxTextFileBytes},
		{"", "book.epub", maxTextFileBytes},
		{"application/octet-stream", "unknown.bin", maxDocumentFileBytes},
	}
	for _, tc := range tests {
		if got := fileSizeLimit(tc.contentType, tc.filename); got != tc.want {
			t.Errorf("fileSizeLimit(%q, %q) = %d, want %d", tc.contentType, tc.filename, got, tc.want)
		}
	}
}

func TestCreateRequestLimits(t *testing.T) {
	longName := strings.Repeat("a", maxFilenameBytes+1) + ".txt"
	req := &CreateRequest{Purpose: PurposeUserData, Filename: longName, File: []byte("x")}
	requireField(t, req.Validate(), "file")

	req = &CreateRequest{
		Purpose:  PurposeUserData,
		Filename: "a.txt",
		File:     make([]byte, maxTextFileBytes+1),
	}
	requireField(t, req.Validate(), "file")

	req = &CreateRequest{Purpose: PurposeUserData, Filename: "a.png", File: make([]byte, maxImageFileBytes+1)}
	requireField(t, req.Validate(), "file")
}

func requireField(t *testing.T, err error, field string) {
	t.Helper()
	var validation *deepseek.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v, want *deepseek.ValidationError", err)
	}
	if validation.Field != field {
		t.Fatalf("field = %q, want %q", validation.Field, field)
	}
}
