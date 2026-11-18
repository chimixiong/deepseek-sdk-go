package tools

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go"
)

const ocrFixture = `{
  "id": "ocr-9b2f4c1e-5d7a-4e3b-8f21-6c0a9d4e7b12",
  "object": "ocr.result",
  "created": 1762790000,
  "purpose": "text_recognition",
  "page_count": 1,
  "pages": [
    {
      "index": 0,
      "text": "DeepSeek API\n\nDeepSeek API 使用与 OpenAI、Anthropic 兼容的 API 格式。",
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "input_tokens": 1536,
    "output_tokens": 412,
    "total_tokens": 1948
  }
}`

func TestOCR(t *testing.T) {
	file := []byte("%PDF-1.7 fake document")
	var (
		order       []string
		gotPurpose  string
		gotFilename string
		gotFile     []byte
	)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tools/ocr" {
			t.Errorf("path = %q", r.URL.Path)
		}
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
			order = append(order, part.FormName())
			switch part.FormName() {
			case "purpose":
				data, _ := io.ReadAll(part)
				gotPurpose = string(data)
			case "file":
				gotFilename = part.FileName()
				gotFile, _ = io.ReadAll(part)
			}
		}
		io.WriteString(w, ocrFixture)
	})

	resp, err := c.OCR(context.Background(), &OCRRequest{
		File:     file,
		Filename: "document.pdf",
		Purpose:  OCRTextRecognition,
	})
	if err != nil {
		t.Fatalf("OCR: %v", err)
	}
	if len(order) != 2 || order[0] != "purpose" || order[1] != "file" {
		t.Fatalf("multipart order = %v, want [purpose file]", order)
	}
	if gotPurpose != "text_recognition" {
		t.Fatalf("purpose = %q", gotPurpose)
	}
	if gotFilename != "document.pdf" {
		t.Fatalf("filename = %q", gotFilename)
	}
	if !bytes.Equal(gotFile, file) {
		t.Fatalf("file = %q, want %q", gotFile, file)
	}

	if resp.ID != "ocr-9b2f4c1e-5d7a-4e3b-8f21-6c0a9d4e7b12" ||
		resp.Object != "ocr.result" ||
		resp.Created != 1762790000 ||
		resp.Purpose != OCRTextRecognition ||
		resp.PageCount != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(resp.Pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(resp.Pages))
	}
	if resp.Pages[0].Index != 0 ||
		resp.Pages[0].Text != "DeepSeek API\n\nDeepSeek API 使用与 OpenAI、Anthropic 兼容的 API 格式。" ||
		resp.Pages[0].FinishReason != OCRFinishStop {
		t.Fatalf("unexpected page: %+v", resp.Pages[0])
	}
	if resp.Usage != (Usage{InputTokens: 1536, OutputTokens: 412, TotalTokens: 1948}) {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestOCRRichLayoutPurpose(t *testing.T) {
	var gotPurpose string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
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
			if part.FormName() == "purpose" {
				data, _ := io.ReadAll(part)
				gotPurpose = string(data)
			}
		}
		io.WriteString(w, `{"id":"ocr-1","object":"ocr.result","created":1,"purpose":"rich_layout_recognition","page_count":0,"pages":[],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}}`)
	})
	_, err := c.OCR(context.Background(), &OCRRequest{
		File:     []byte("img"),
		Filename: "page.png",
		Purpose:  OCRRichLayoutRecognition,
	})
	if err != nil {
		t.Fatalf("OCR: %v", err)
	}
	if gotPurpose != "rich_layout_recognition" {
		t.Fatalf("purpose = %q", gotPurpose)
	}
}

func TestOCRValidation(t *testing.T) {
	c := newTestClient(t, noRequest(t))
	tests := []struct {
		name  string
		req   *OCRRequest
		field string
	}{
		{"nil request", nil, "request"},
		{"empty file", &OCRRequest{Filename: "a.pdf", Purpose: OCRTextRecognition}, "file"},
		{"empty filename", &OCRRequest{File: []byte("x"), Purpose: OCRTextRecognition}, "filename"},
		{"empty purpose", &OCRRequest{File: []byte("x"), Filename: "a.pdf"}, "purpose"},
		{"unknown purpose", &OCRRequest{File: []byte("x"), Filename: "a.pdf", Purpose: "bogus"}, "purpose"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.OCR(context.Background(), tt.req)
			requireValidationError(t, err, tt.field)
		})
	}
}

func TestOCRLongFinishReason(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":"ocr-1","object":"ocr.result","created":1,"purpose":"text_recognition","page_count":1,"pages":[{"index":0,"text":"cut","finish_reason":"length"}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`)
	})
	resp, err := c.OCR(context.Background(), &OCRRequest{
		File:     []byte("x"),
		Filename: "a.pdf",
		Purpose:  OCRTextRecognition,
	})
	if err != nil {
		t.Fatalf("OCR: %v", err)
	}
	if resp.Pages[0].FinishReason != OCRFinishLength {
		t.Fatalf("finish_reason = %q, want length", resp.Pages[0].FinishReason)
	}
}

func TestOCRReturnsGenerationError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `
{"error":{"message":"OCR did not finish in time","type":"gateway_timeout_error","param":null,"code":"gateway_timeout_error"}}`)
	})
	resp, err := c.OCR(context.Background(), &OCRRequest{
		File:     []byte("fake"),
		Filename: "document.pdf",
		Purpose:  OCRTextRecognition,
	})
	var generationErr *deepseek.ResponseError
	if !errors.As(err, &generationErr) {
		t.Fatalf("err = %v, want *deepseek.ResponseError", err)
	}
	if generationErr.Code != "gateway_timeout_error" {
		t.Fatalf("generation error = %+v", generationErr)
	}
	if resp == nil {
		t.Fatal("response must be returned alongside the error")
	}
}
