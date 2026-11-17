// Package files implements the DeepSeek file upload API: creating, listing,
// retrieving and deleting file objects.
package files

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Client calls the files API.
type Client struct {
	api *deepseek.Client
}

// New returns a files client backed by the shared transport. A nil core
// client is replaced with a default one.
func New(c *deepseek.Client) *Client {
	if c == nil {
		c = deepseek.New(deepseek.Config{})
	}
	return &Client{api: c}
}

// Create uploads a file. The purpose part is sent before the file part as the
// API requires.
func (c *Client) Create(ctx context.Context, req *CreateRequest) (*File, error) {
	if req == nil {
		return nil, deepseek.Invalid("request", "must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	parts := []deepseek.MultipartPart{
		{Field: "purpose", Value: []byte(string(req.Purpose))},
		{
			Field:       "file",
			Filename:    req.Filename,
			ContentType: req.ContentType,
			Value:       req.File,
		},
	}
	if req.ExpiresAfter != nil {
		anchor := req.ExpiresAfter.Anchor
		if anchor == "" {
			anchor = AnchorCreatedAt
		}
		parts = append(parts,
			deepseek.MultipartPart{Field: "expires_after[anchor]", Value: []byte(string(anchor))},
			deepseek.MultipartPart{Field: "expires_after[seconds]", Value: []byte(strconv.Itoa(req.ExpiresAfter.Seconds))},
		)
	}
	var file File
	if err := c.api.DoJSON(ctx, &deepseek.Request{
		URL:       c.api.FilesURL(),
		Multipart: parts,
	}, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

// List returns one page of files. A nil options value lists with server
// defaults.
func (c *Client) List(ctx context.Context, options *ListOptions) (*ListResponse, error) {
	query := url.Values{}
	if options != nil {
		if err := options.validate(); err != nil {
			return nil, err
		}
		if options.After != "" {
			query.Set("after", options.After)
		}
		if options.Limit != nil {
			query.Set("limit", strconv.Itoa(*options.Limit))
		}
		if options.Order != "" {
			query.Set("order", string(options.Order))
		}
		if options.Purpose != "" {
			query.Set("purpose", string(options.Purpose))
		}
	}
	var list ListResponse
	if err := c.api.DoJSON(ctx, &deepseek.Request{
		Method: http.MethodGet,
		URL:    c.api.FilesURL(),
		Query:  query,
	}, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// Retrieve returns the metadata for one file.
func (c *Client) Retrieve(ctx context.Context, id string) (*File, error) {
	if id == "" {
		return nil, deepseek.Invalid("file_id", "is required")
	}
	var file File
	if err := c.api.DoJSON(ctx, &deepseek.Request{
		Method: http.MethodGet,
		URL:    c.api.FileURL(id),
	}, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

// Delete removes one file.
func (c *Client) Delete(ctx context.Context, id string) (*DeleteResponse, error) {
	if id == "" {
		return nil, deepseek.Invalid("file_id", "is required")
	}
	var deleted DeleteResponse
	if err := c.api.DoJSON(ctx, &deepseek.Request{
		Method: http.MethodDelete,
		URL:    c.api.FileURL(id),
	}, &deleted); err != nil {
		return nil, err
	}
	return &deleted, nil
}
