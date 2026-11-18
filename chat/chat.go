// Package chat implements the DeepSeek Chat Completions API.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Client calls the Chat Completions endpoint.
type Client struct {
	core *deepseek.Client
	url  string
	beta bool
}

// New returns a client for the standard endpoint. A nil client falls back to a
// default client, matching messages.New.
func New(c *deepseek.Client) *Client {
	if c == nil {
		c = deepseek.New(deepseek.Config{})
	}
	return &Client{core: c, url: c.ChatCompletionsURL()}
}

// NewBeta returns a client for the beta endpoint, required when an assistant
// message uses Prefix continuation. A nil client falls back to a default
// client.
func NewBeta(c *deepseek.Client) *Client {
	if c == nil {
		c = deepseek.New(deepseek.Config{})
	}
	return &Client{core: c, url: c.BetaChatCompletionsURL(), beta: true}
}

// Create sends a non-streaming request and returns the first choice response.
// A 200 response whose body carries a generation error is returned with that
// error, so the billed usage stays readable.
func (c *Client) Create(ctx context.Context, req *Request) (*Response, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := c.validateTransport(req); err != nil {
		return nil, err
	}
	wire := req.prepared(false)
	var resp Response
	if err := c.core.DoJSON(ctx, &deepseek.Request{URL: c.url, JSON: wire}, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return &resp, resp.Error
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("deepseek/chat: response has no choices")
	}
	return &resp, nil
}

// CreateStream sends a streaming request and yields chunks until [DONE].
// Usage is requested on the final chunk. Validation errors are yielded by the
// iterator before any request is made.
func (c *Client) CreateStream(ctx context.Context, req *Request) iter.Seq2[Chunk, error] {
	return func(yield func(Chunk, error) bool) {
		if err := req.validate(true); err != nil {
			yield(Chunk{}, err)
			return
		}
		if err := c.validateTransport(req); err != nil {
			yield(Chunk{}, err)
			return
		}
		wire := req.prepared(true)
		stream, err := c.core.OpenStream(ctx, &deepseek.Request{URL: c.url, JSON: wire})
		if err != nil {
			yield(Chunk{}, err)
			return
		}
		defer stream.Close()
		done := false
		for {
			event, err := stream.Next()
			if err == io.EOF {
				if !done {
					yield(Chunk{}, errors.New("deepseek/chat: stream ended before [DONE]"))
				}
				return
			}
			if err != nil {
				yield(Chunk{}, err)
				return
			}
			if string(event.Data) == "[DONE]" {
				done = true
				return
			}
			var chunk Chunk
			if err := json.Unmarshal(event.Data, &chunk); err != nil {
				yield(Chunk{}, fmt.Errorf("deepseek/chat: decode chunk: %w", err))
				return
			}
			if chunk.Error != nil {
				yield(chunk, chunk.Error)
				return
			}
			if !yield(chunk, nil) {
				return
			}
		}
	}
}

// Accumulate drains a stream into one assembled response. On error it returns
// the partial response assembled so far, so usage from a failed generation
// stays readable, matching messages.Aggregate.
func Accumulate(seq iter.Seq2[Chunk, error]) (*Response, error) {
	if seq == nil {
		return nil, deepseek.Invalid("stream", "must not be nil")
	}
	acc := &StreamAccumulator{}
	for chunk, err := range seq {
		acc.Add(chunk)
		if err != nil {
			return acc.Response(), err
		}
	}
	return acc.Response(), nil
}

func (c *Client) validateTransport(req *Request) error {
	if c.beta {
		return nil
	}
	for i, message := range req.Messages {
		if message.Prefix != nil && *message.Prefix {
			return deepseek.Invalid(fmt.Sprintf("messages[%d].prefix", i), "requires a beta client (chat.NewBeta)")
		}
	}
	return nil
}
