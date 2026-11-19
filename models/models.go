// Package models implements the DeepSeek model listing API.
package models

import (
	"context"
	"errors"
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
	client, err := c.transport()
	if err != nil {
		return nil, err
	}
	var resp ListResponse
	if err := client.DoJSON(ctx, &deepseek.Request{
		Method: http.MethodGet,
		URL:    client.ModelsURL(),
	}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) transport() (*deepseek.Client, error) {
	if c == nil || c.api == nil {
		return nil, errors.New("models: nil deepseek client")
	}
	return c.api, nil
}
