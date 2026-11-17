package tools

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
)

func TestASR(t *testing.T) {
	file := []byte("OggS fake opus payload")
	var (
		gotPath     string
		gotField    string
		gotFilename string
		gotFile     []byte
	)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("multipart reader: %v", err)
		}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("next part: %v", err)
			}
			gotField = part.FormName()
			gotFilename = part.FileName()
			gotFile, _ = io.ReadAll(part)
		}
		io.WriteString(w, `{
			"request_id": "0b8c1c7e-3c0e-4d0f-9d3a-5f2a6f7e8b90",
			"text": "今天上海的天气怎么样？",
			"duration_ms": 3200
		}`)
	})

	resp, err := c.ASR(context.Background(), &ASRRequest{File: file, Filename: "speech.ogg"})
	if err != nil {
		t.Fatalf("ASR: %v", err)
	}
	if gotPath != "/v1/tools/asr" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotField != "file" || gotFilename != "speech.ogg" {
		t.Fatalf("part = %q/%q, want file/speech.ogg", gotField, gotFilename)
	}
	if !bytes.Equal(gotFile, file) {
		t.Fatalf("file = %q, want %q", gotFile, file)
	}
	if resp.RequestID != "0b8c1c7e-3c0e-4d0f-9d3a-5f2a6f7e8b90" ||
		resp.Text != "今天上海的天气怎么样？" ||
		resp.DurationMS != 3200 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestASRValidation(t *testing.T) {
	c := newTestClient(t, noRequest(t))
	tests := []struct {
		name  string
		req   *ASRRequest
		field string
	}{
		{"nil request", nil, "request"},
		{"empty file", &ASRRequest{Filename: "a.wav"}, "file"},
		{"empty filename", &ASRRequest{File: []byte("x")}, "filename"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.ASR(context.Background(), tt.req)
			requireValidationError(t, err, tt.field)
		})
	}
}

func TestASRAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"message":"unsupported audio format","type":"invalid_request_error"}}`)
	})
	_, err := c.ASR(context.Background(), &ASRRequest{File: []byte("x"), Filename: "a.txt"})
	apiErr := requireAPIError(t, err, http.StatusBadRequest)
	if apiErr.Message != "unsupported audio format" {
		t.Fatalf("message = %q", apiErr.Message)
	}
}
