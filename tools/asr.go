package tools

import (
	"context"
	"net/http"

	"github.com/chimixiong/deepseek-sdk-go"
)

// ASRRequest is the multipart body of POST /v1/tools/asr.
type ASRRequest struct {
	// File is a 16-bit PCM WAV or Ogg Opus audio payload.
	File []byte `json:"-"`
	// Filename is the name sent with File. The API requires a file part, which
	// needs a name.
	Filename string `json:"-"`
}

// Validate reports the documented request constraints.
func (r *ASRRequest) Validate() error {
	if r == nil {
		return deepseek.Invalid("request", "must not be nil")
	}
	if len(r.File) == 0 {
		return deepseek.Invalid("file", "must not be empty")
	}
	if r.Filename == "" {
		return deepseek.Invalid("filename", "must not be empty")
	}
	return nil
}

// ASRResponse is the response of POST /v1/tools/asr.
type ASRResponse struct {
	// RequestID identifies the request.
	RequestID string `json:"request_id"`
	// Text is the recognized transcript.
	Text string `json:"text"`
	// DurationMS is the audio duration in milliseconds.
	DurationMS int64 `json:"duration_ms"`
}

// ASR transcribes one complete audio file.
func (c *Client) ASR(ctx context.Context, req *ASRRequest) (*ASRResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	client, err := c.transport()
	if err != nil {
		return nil, err
	}
	var out ASRResponse
	err = client.DoJSON(ctx, &deepseek.Request{
		Method: http.MethodPost,
		URL:    client.ToolURL("asr"),
		Multipart: []deepseek.MultipartPart{
			{Field: "file", Filename: req.Filename, Value: req.File},
		},
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
