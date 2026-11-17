// Package models implements the DeepSeek model listing API.
package models

import (
	"context"
	"net/http"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Client calls the models API.
type Client struct {
	api *deepseek.Client
}

// New returns a models client backed by the shared transport. A nil core
// client is replaced with a default one.
func New(c *deepseek.Client) *Client {
	if c == nil {
		c = deepseek.New(deepseek.Config{})
	}
	return &Client{api: c}
}

// List returns the models available to the account.
func (c *Client) List(ctx context.Context) (*ListResponse, error) {
	var resp ListResponse
	if err := c.api.DoJSON(ctx, &deepseek.Request{
		Method: http.MethodGet,
		URL:    c.api.ModelsURL(),
	}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
