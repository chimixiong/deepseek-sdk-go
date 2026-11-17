package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"strings"
)

// Request is one HTTP call. Set at most one of JSON, Multipart or Body; a
// request without a payload (GET, DELETE) leaves all three nil.
type Request struct {
	// Method defaults to POST.
	Method string
	// URL is the absolute endpoint.
	URL string
	// Header is optional; each name replaces the SDK default entirely, so a
	// caller can override Content-Type, Accept or Authorization. An empty value
	// list removes the header.
	Header http.Header
	// Query is appended to the URL.
	Query url.Values
	// JSON is marshalled as the request body.
	JSON any
	// Multipart is encoded in order, so fields that the API requires before a
	// file (for example "purpose") stay first.
	Multipart []MultipartPart
	// Body is sent verbatim; ContentType defaults to application/octet-stream.
	Body        []byte
	ContentType string
}

// MultipartPart is one form-data part.
type MultipartPart struct {
	// Field is the form field name.
	Field string
	// Filename marks the part as a file upload.
	Filename string
	// ContentType defaults to application/octet-stream for files and is
	// omitted for plain fields.
	ContentType string
	// Value is the part body.
	Value []byte
}

// HeaderValue returns a header from the request.
func (r *Request) HeaderValue(name string) string {
	if r.Header == nil {
		return ""
	}
	return r.Header.Get(name)
}

// Do sends the request and returns the 2xx response with its body unread.
// The caller owns the body and must close it. Retries 429/5xx and transport
// errors raised before the request was written; a retried body is replayed
// from encoded bytes. A dropped connection after the request went out is not
// retried for a non-idempotent method, because the server may have acted on
// it.
func (c *Client) Do(ctx context.Context, req *Request) (*http.Response, error) {
	return c.do(ctx, req, false)
}

// DoJSON sends the request and decodes the JSON response into out. A nil out
// discards the body.
func (c *Client) DoJSON(ctx context.Context, req *Request, out any) error {
	resp, err := c.do(ctx, req, false)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		// A 2xx response with no body (204, empty 200) is success.
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

// OpenStream sends the request and returns the server-sent event stream.
// Retries the initial call only; once events flow, errors surface from
// Stream.Next. The caller must Close the stream.
func (c *Client) OpenStream(ctx context.Context, req *Request) (*Stream, error) {
	resp, err := c.do(ctx, req, true)
	if err != nil {
		return nil, err
	}
	return NewStream(resp.Body), nil
}

func (c *Client) do(ctx context.Context, req *Request, stream bool) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("deepseek: nil request")
	}
	target, err := requestURL(req)
	if err != nil {
		return nil, err
	}
	method := req.Method
	if method == "" {
		method = http.MethodPost
	}
	payload, contentType, err := encodeBody(req)
	if err != nil {
		return nil, err
	}

	for attempt := 0; ; attempt++ {
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}
		wrote := false
		trace := &httptrace.ClientTrace{
			WroteRequest: func(info httptrace.WroteRequestInfo) {
				if info.Err == nil {
					wrote = true
				}
			},
		}
		httpReq, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), method, target, body)
		if err != nil {
			return nil, err
		}
		if contentType != "" {
			httpReq.Header.Set("Content-Type", contentType)
		}
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		if c.userAgent != "" {
			httpReq.Header.Set("User-Agent", c.userAgent)
		}
		switch {
		case stream:
			httpReq.Header.Set("Accept", "text/event-stream")
		case req.HeaderValue("Accept") == "":
			httpReq.Header.Set("Accept", "application/json")
		}
		for name, values := range req.Header {
			key := textproto.CanonicalMIMEHeaderKey(name)
			if len(values) == 0 {
				httpReq.Header.Del(key)
				continue
			}
			httpReq.Header[key] = append([]string(nil), values...)
		}

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			if attempt < c.maxRetries && ctx.Err() == nil && (!wrote || idempotentMethod(method)) {
				if !sleep(ctx, c.backoff(attempt)) {
					return nil, ctx.Err()
				}
				continue
			}
			return nil, &TransportError{Err: err}
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		retryAfter := resp.Header.Get("Retry-After")
		apiErr := readAPIError(resp)
		if attempt < c.maxRetries && retryableStatus(resp.StatusCode) {
			delay := c.backoff(attempt)
			if retryAfter != "" {
				if d, ok := parseRetryAfter(retryAfter, now()); ok {
					delay = d
				}
			}
			if !sleep(ctx, delay) {
				return nil, ctx.Err()
			}
			continue
		}
		return nil, apiErr
	}
}

func requestURL(req *Request) (string, error) {
	if req.URL == "" {
		return "", errors.New("deepseek: request URL is empty")
	}
	u, err := url.Parse(req.URL)
	if err != nil {
		return "", fmt.Errorf("deepseek: invalid URL: %w", err)
	}
	if len(req.Query) > 0 {
		query := u.Query()
		for key, values := range req.Query {
			for _, value := range values {
				query.Add(key, value)
			}
		}
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}

func encodeBody(req *Request) ([]byte, string, error) {
	count := 0
	if req.JSON != nil {
		count++
	}
	if len(req.Multipart) > 0 {
		count++
	}
	if req.Body != nil {
		count++
	}
	if count > 1 {
		return nil, "", errors.New("deepseek: request has more than one payload")
	}
	switch {
	case req.JSON != nil:
		payload, err := json.Marshal(req.JSON)
		if err != nil {
			return nil, "", fmt.Errorf("deepseek: encode JSON: %w", err)
		}
		return payload, "application/json", nil
	case len(req.Multipart) > 0:
		payload, contentType, err := encodeMultipart(req.Multipart)
		return payload, contentType, err
	case req.Body != nil:
		contentType := req.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		return req.Body, contentType, nil
	default:
		return nil, "", nil
	}
}

func encodeMultipart(parts []MultipartPart) ([]byte, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for _, part := range parts {
		if part.Field == "" {
			writer.Close()
			return nil, "", errors.New("deepseek: multipart part without a field name")
		}
		header := make(textproto.MIMEHeader)
		disposition := `form-data; name="` + escapeQuotes(part.Field) + `"`
		if part.Filename != "" {
			disposition += `; filename="` + escapeQuotes(part.Filename) + `"`
		}
		header.Set("Content-Disposition", disposition)
		if part.ContentType != "" {
			header.Set("Content-Type", part.ContentType)
		} else if part.Filename != "" {
			header.Set("Content-Type", "application/octet-stream")
		}
		dst, err := writer.CreatePart(header)
		if err != nil {
			writer.Close()
			return nil, "", err
		}
		if _, err := dst.Write(part.Value); err != nil {
			writer.Close()
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

func escapeQuotes(s string) string {
	// Same convention as mime/multipart: CR and LF are percent-encoded so a
	// caller-supplied field or file name cannot inject a part header.
	return strings.NewReplacer(
		"\\", "\\\\",
		`"`, `\"`,
		"\r", "%0D",
		"\n", "%0A",
	).Replace(s)
}

// idempotentMethod reports whether replaying a request whose body already
// reached the server is safe.
func idempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}
