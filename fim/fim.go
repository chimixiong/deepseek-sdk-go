// Package fim implements the DeepSeek Fill-In-the-Middle completion API.
//
// The API lives under the beta root and returns text completions where the
// prompt can be completed at an insertion point specified by suffix.
package fim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Client calls the FIM API.
type Client struct {
	api *deepseek.Client
}

// New returns a FIM client backed by the shared transport. A nil core client
// is replaced with a default one.
func New(c *deepseek.Client) *Client {
	if c == nil {
		c = deepseek.New(deepseek.Config{})
	}
	return &Client{api: c}
}

// Create sends a non-streaming completion request.
func (c *Client) Create(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, deepseek.Invalid("request", "must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.Stream != nil && *req.Stream {
		return nil, deepseek.Invalid("stream", "must be false; use CreateStream for streaming")
	}
	if req.StreamOptions != nil {
		return nil, deepseek.Invalid("stream_options", "requires CreateStream")
	}
	client, err := c.transport()
	if err != nil {
		return nil, err
	}
	body := *req
	body.Stream = nil
	body.StreamOptions = nil
	var resp Response
	if err := client.DoJSON(ctx, &deepseek.Request{
		URL:  client.FIMURL(),
		JSON: &body,
	}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateStream sends a streaming completion request. The request is copied and
// stream is enabled with include_usage so the last chunk carries token usage.
func (c *Client) CreateStream(ctx context.Context, req *Request) (*Stream, error) {
	if req == nil {
		return nil, deepseek.Invalid("request", "must not be nil")
	}
	client, err := c.transport()
	if err != nil {
		return nil, err
	}
	body := *req
	body.Stream = new(true)
	options := StreamOptions{}
	if req.StreamOptions != nil {
		options = *req.StreamOptions
	}
	options.IncludeUsage = new(true)
	body.StreamOptions = &options
	if err := body.Validate(); err != nil {
		return nil, err
	}
	stream, err := client.OpenStream(ctx, &deepseek.Request{
		URL:  client.FIMURL(),
		JSON: &body,
	})
	if err != nil {
		return nil, err
	}
	return &Stream{stream: stream}, nil
}

// Stream delivers FIM completion chunks. Each chunk is the same structure as a
// non-streaming response with partial choice text.
type Stream struct {
	stream *deepseek.Stream
	done   bool
}

// Next returns the next chunk, or io.EOF after the [DONE] terminator. A stream
// that ends before [DONE], for example because the connection dropped, returns
// an error instead of io.EOF so a truncated completion is never mistaken for a
// complete one.
func (s *Stream) Next() (*Response, error) {
	for {
		event, err := s.stream.Next()
		if err == io.EOF && !s.done {
			return nil, errors.New("deepseek/fim: stream ended before [DONE]")
		}
		if err != nil {
			return nil, err
		}
		if string(event.Data) == "[DONE]" {
			s.done = true
			return nil, io.EOF
		}
		if len(event.Data) == 0 {
			continue
		}
		var chunk Response
		if err := json.Unmarshal(event.Data, &chunk); err != nil {
			return nil, fmt.Errorf("fim: decode stream event: %w", err)
		}
		return &chunk, nil
	}
}

// Close releases the underlying response body.
func (s *Stream) Close() error { return s.stream.Close() }

func (c *Client) transport() (*deepseek.Client, error) {
	if c == nil || c.api == nil {
		return nil, errors.New("fim: nil deepseek client")
	}
	return c.api, nil
}
