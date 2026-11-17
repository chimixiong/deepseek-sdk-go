// Package messages implements the Anthropic-compatible Messages API at
// POST /anthropic/v1/messages.
//
// Requests are authenticated with Authorization: Bearer (added by the shared
// transport) and x-api-key. Responses use Anthropic content blocks; thinking
// signatures are preserved verbatim so they can be echoed back on tool turns.
package messages

import (
	"context"
	"errors"
	"io"
	"iter"
	"net/http"

	deepseek "github.com/chimixiong/deepseek-sdk-go"
)

// APIVersion is sent as the anthropic-version header. The server ignores it but
// Anthropic clients expect it.
const APIVersion = "2023-06-01"

// FilesBeta is the anthropic-beta header value required when an image or
// document source references an uploaded file.
const FilesBeta = "files-api-2025-04-14"

// Client calls the Messages API.
type Client struct {
	api *deepseek.Client
}

// New returns a Messages client backed by the shared transport. A nil core
// client is replaced with a default one.
func New(c *deepseek.Client) *Client {
	if c == nil {
		c = deepseek.New(deepseek.Config{})
	}
	return &Client{api: c}
}

// Create sends a non-streaming request and returns the decoded message.
// It rejects requests with Stream set; use CreateStream instead.
func (c *Client) Create(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, deepseek.Invalid("request", "must not be nil")
	}
	if req.Stream {
		return nil, deepseek.Invalid("stream", "must be false; use CreateStream for streaming")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var out Response
	if err := c.api.DoJSON(ctx, c.httpRequest(req), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateStream sends a streaming request and yields its events. Stream is
// forced to true. Validation, connection and decoding failures are yielded as
// errors and stop the sequence; io.EOF ends it without an error.
//
// A complete stream ends with message_stop and never sends [DONE]. Use
// Aggregate to assemble the final message.
func (c *Client) CreateStream(ctx context.Context, req *Request) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		if req == nil {
			yield(Event{}, deepseek.Invalid("request", "must not be nil"))
			return
		}
		body := *req
		body.Stream = true
		if err := body.Validate(); err != nil {
			yield(Event{}, err)
			return
		}
		stream, err := c.api.OpenStream(ctx, c.httpRequest(&body))
		if err != nil {
			yield(Event{}, err)
			return
		}
		defer stream.Close()
		for {
			raw, err := stream.Next()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				yield(Event{}, err)
				return
			}
			event, ok, err := decodeEvent(raw)
			if err != nil {
				yield(Event{}, err)
				return
			}
			if !ok {
				continue
			}
			if !yield(event, nil) {
				return
			}
		}
	}
}

func (c *Client) httpRequest(req *Request) *deepseek.Request {
	header := http.Header{}
	header.Set("anthropic-version", APIVersion)
	if key := c.api.APIKey(); key != "" {
		header.Set("x-api-key", key)
	}
	if needsFilesBeta(req) {
		header.Set("anthropic-beta", FilesBeta)
	}
	return &deepseek.Request{
		URL:    c.api.MessagesURL(),
		Header: header,
		JSON:   req,
	}
}

// needsFilesBeta reports whether any image or document source references an
// uploaded file, which requires the files-api beta header.
func needsFilesBeta(req *Request) bool {
	if req == nil {
		return false
	}
	for _, message := range req.Messages {
		if contentUsesFiles(message.Content) {
			return true
		}
	}
	return false
}

func contentUsesFiles(content Content) bool {
	for _, block := range content.Blocks {
		if block.Source != nil && block.Source.Type == SourceFile {
			return true
		}
		if block.Content != nil && contentUsesFiles(*block.Content) {
			return true
		}
	}
	return false
}
