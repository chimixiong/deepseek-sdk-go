// Package responses implements the DeepSeek Responses API.
//
// The API is stateless: every request carries the full conversation in input.
// Responses are either returned whole (Create) or as semantic server-sent
// events whose terminal event is response.completed, response.incomplete or
// response.failed (CreateStream). There is no data: [DONE] marker.
package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"reflect"
	"strings"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Client calls the Responses endpoint.
type Client struct {
	core *deepseek.Client
	url  string
}

// New returns a client for the Responses endpoint. A nil core uses the default
// configuration.
func New(c *deepseek.Client) *Client {
	if c == nil {
		c = deepseek.New(deepseek.Config{})
	}
	return &Client{core: c, url: c.ResponsesURL()}
}

// Create sends a non-streaming request. Stream is forced to false, overriding
// any value in req.
func (c *Client) Create(ctx context.Context, req *Request) (*Response, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var resp Response
	if err := c.core.DoJSON(ctx, &deepseek.Request{URL: c.url, JSON: req.prepared(false)}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateStream sends a streaming request and yields semantic events until the
// server closes the stream. Stream is forced to true, overriding any value in
// req. Validation errors are yielded before any request is made.
func (c *Client) CreateStream(ctx context.Context, req *Request) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		if err := req.Validate(); err != nil {
			yield(Event{}, err)
			return
		}
		stream, err := c.core.OpenStream(ctx, &deepseek.Request{URL: c.url, JSON: req.prepared(true)})
		if err != nil {
			yield(Event{}, err)
			return
		}
		defer stream.Close()
		for {
			sse, err := stream.Next()
			if err != nil {
				if err == io.EOF {
					return
				}
				yield(Event{}, err)
				return
			}
			if len(sse.Data) == 0 {
				continue
			}
			if string(sse.Data) == "[DONE]" {
				return
			}
			var event Event
			if err := json.Unmarshal(sse.Data, &event); err != nil {
				yield(Event{}, fmt.Errorf("deepseek/responses: decode event: %w", err))
				return
			}
			if sse.Event != "" {
				event.Type = EventType(sse.Event)
			}
			if !yield(event, nil) {
				return
			}
		}
	}
}

// splitExtra returns the top-level JSON fields of data that are not modelled by
// sample's exported fields. It lets response types preserve fields added to the
// protocol after this release.
func splitExtra(data []byte, sample any) (map[string]json.RawMessage, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	t := reflect.TypeOf(sample)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		delete(all, name)
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all, nil
}
