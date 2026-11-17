// Package tools implements the DeepSeek tool endpoints: web search, web fetch,
// OCR, text-to-speech and speech-to-text.
//
// Every method takes a typed request, validates the documented constraints
// before any network call and returns either the decoded response, a streamed
// audio handle, a *deepseek.ValidationError or an error from the transport.
package tools

import (
	"errors"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Client groups the tool endpoints behind one transport client.
type Client struct {
	client *deepseek.Client
}

// New returns a tools client that sends requests through the shared
// transport. A nil core client is replaced with a default one.
func New(c *deepseek.Client) *Client {
	if c == nil {
		c = deepseek.New(deepseek.Config{})
	}
	return &Client{client: c}
}

// DeepSeek returns the underlying transport client.
func (c *Client) DeepSeek() *deepseek.Client {
	if c == nil {
		return nil
	}
	return c.client
}

func (c *Client) transport() (*deepseek.Client, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("tools: nil deepseek client")
	}
	return c.client, nil
}
