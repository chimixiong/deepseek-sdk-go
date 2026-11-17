// Package billing implements the DeepSeek account balance API.
package billing

import (
	"context"
	"net/http"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Client calls the billing API.
type Client struct {
	api *deepseek.Client
}

// New returns a billing client backed by the shared transport. A nil core
// client is replaced with a default one.
func New(c *deepseek.Client) *Client {
	if c == nil {
		c = deepseek.New(deepseek.Config{})
	}
	return &Client{api: c}
}

// GetBalance returns the account balance and availability.
func (c *Client) GetBalance(ctx context.Context) (*Balance, error) {
	var balance Balance
	if err := c.api.DoJSON(ctx, &deepseek.Request{
		Method: http.MethodGet,
		URL:    c.api.BalanceURL(),
	}, &balance); err != nil {
		return nil, err
	}
	return &balance, nil
}
