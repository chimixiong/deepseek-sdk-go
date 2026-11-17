package tools

import (
	"context"
	"net/http"

	"github.com/chimixiong/deepseek-sdk-go"
)

// OCRPurpose selects how OCR reads a document.
type OCRPurpose string

const (
	// OCRTextRecognition extracts text directly from image-free Word and
	// PowerPoint files and recognizes all other files page by page.
	OCRTextRecognition OCRPurpose = "text_recognition"
	// OCRRichLayoutRecognition recognizes every file page by page.
	OCRRichLayoutRecognition OCRPurpose = "rich_layout_recognition"
)

// OCRFinishReason reports whether a page was recognized completely.
type OCRFinishReason string

const (
	OCRFinishStop   OCRFinishReason = "stop"
	OCRFinishLength OCRFinishReason = "length"
)

// OCRRequest is the multipart body of POST /v1/tools/ocr.
type OCRRequest struct {
	// File is the document or image payload.
	File []byte `json:"-"`
	// Filename is the name sent with File. The API requires it.
	Filename string `json:"-"`
	// Purpose is required.
	Purpose OCRPurpose `json:"purpose"`
}

// Validate reports the documented request constraints.
func (r *OCRRequest) Validate() error {
	if r == nil {
		return deepseek.Invalid("request", "must not be nil")
	}
	if len(r.File) == 0 {
		return deepseek.Invalid("file", "must not be empty")
	}
	if r.Filename == "" {
		return deepseek.Invalid("filename", "must not be empty")
	}
	if r.Purpose == "" {
		return deepseek.Invalid("purpose", "must not be empty")
	}
	if r.Purpose != OCRTextRecognition && r.Purpose != OCRRichLayoutRecognition {
		return deepseek.Invalid("purpose", "must be %q or %q", OCRTextRecognition, OCRRichLayoutRecognition)
	}
	return nil
}

// OCRResponse is the response of POST /v1/tools/ocr.
type OCRResponse struct {
	ID      string     `json:"id"`
	Object  string     `json:"object"`
	Created int64      `json:"created"`
	Purpose OCRPurpose `json:"purpose"`
	// PageCount equals len(Pages).
	PageCount int    `json:"page_count"`
	Pages     []Page `json:"pages"`
	Usage     Usage  `json:"usage"`
}

// Page is one recognized page.
type Page struct {
	// Index starts at 0.
	Index int `json:"index"`
	// Text is the raw model output for the page.
	Text string `json:"text"`
	// FinishReason is OCRFinishLength when the page was truncated.
	FinishReason OCRFinishReason `json:"finish_reason"`
}

// Usage is the token usage across all pages. It is informational and not used
// for billing.
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

// OCR recognizes text in a PDF, Word, PowerPoint or image file. The file is
// not stored in the Files API.
func (c *Client) OCR(ctx context.Context, req *OCRRequest) (*OCRResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	client, err := c.transport()
	if err != nil {
		return nil, err
	}
	var out OCRResponse
	err = client.DoJSON(ctx, &deepseek.Request{
		Method: http.MethodPost,
		URL:    client.ToolURL("ocr"),
		Multipart: []deepseek.MultipartPart{
			{Field: "purpose", Value: []byte(req.Purpose)},
			{Field: "file", Filename: req.Filename, Value: req.File},
		},
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
