package deepseek

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxSSELine bounds one input line so a hostile stream cannot grow memory
// without limit.
const maxSSELine = 1 << 20

// SSEEvent is one dispatched server-sent event.
type SSEEvent struct {
	// ID is the optional event id field.
	ID string
	// Event is the optional event name field. Chat and FIM leave it empty; the
	// Responses and Messages APIs use it for semantic events.
	Event string
	// Data is the joined data field as raw JSON. The terminator "[DONE]" is
	// delivered verbatim; each API decides how its stream ends.
	Data []byte
	// Retry is the reconnection hint, when the server sent one.
	Retry time.Duration
}

// Stream reads server-sent events from an HTTP response body.
type Stream struct {
	body       io.ReadCloser
	r          *bufio.Reader
	done       bool
	bomChecked bool
}

// NewStream wraps body. Call Close when finished. A nil body yields an empty
// stream.
func NewStream(body io.ReadCloser) *Stream {
	if body == nil {
		return &Stream{}
	}
	return &Stream{body: body, r: bufio.NewReaderSize(body, 64<<10)}
}

// Next returns the next event, or io.EOF at the end of the stream. Comment
// lines and events whose data payload is empty are skipped.
func (s *Stream) Next() (SSEEvent, error) {
	if s.done || s.r == nil {
		return SSEEvent{}, io.EOF
	}
	var (
		ev   SSEEvent
		data []string
	)
	for {
		line, err := s.readLine()
		if !s.bomChecked {
			line = strings.TrimPrefix(line, "\uFEFF")
			s.bomChecked = true
		}
		if line != "" {
			if strings.HasPrefix(line, ":") {
				// Comment, often a keep-alive.
			} else {
				field, value, found := strings.Cut(line, ":")
				if found {
					value = strings.TrimPrefix(value, " ")
				}
				switch field {
				case "data":
					data = append(data, value)
				case "event":
					ev.Event = value
				case "id":
					ev.ID = value
				case "retry":
					if ms, convErr := strconv.Atoi(value); convErr == nil {
						ev.Retry = time.Duration(ms) * time.Millisecond
					}
				}
			}
			if err == nil {
				continue
			}
		}
		if err != nil {
			if err == io.EOF {
				// The SSE spec discards an event that is not terminated by a
				// blank line.
				s.done = true
				return SSEEvent{}, io.EOF
			}
			return SSEEvent{}, err
		}
		// Blank line: dispatch. A payload that is empty, either because no data
		// field was seen or because every data field was empty, is not an event;
		// the spec resets the buffers.
		payload := strings.Join(data, "\n")
		if payload == "" {
			ev = SSEEvent{}
			data = nil
			continue
		}
		ev.Data = []byte(payload)
		return ev, nil
	}
}

// readLine returns one line without its terminator. LF, CRLF and a bare CR all
// terminate a line, and a final line without any terminator is returned before
// io.EOF.
func (s *Stream) readLine() (string, error) {
	var line []byte
	for {
		b, err := s.r.ReadByte()
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				return string(line), nil
			}
			return "", err
		}
		switch b {
		case '\n':
			return string(line), nil
		case '\r':
			if next, peekErr := s.r.Peek(1); peekErr == nil && next[0] == '\n' {
				_, _ = s.r.Discard(1)
			}
			return string(line), nil
		default:
			if len(line) >= maxSSELine {
				return "", fmt.Errorf("deepseek: SSE line exceeds %d bytes", maxSSELine)
			}
			line = append(line, b)
		}
	}
}

// Close releases the underlying response body.
func (s *Stream) Close() error {
	if s.body == nil {
		return nil
	}
	err := s.body.Close()
	s.body = nil
	return err
}

// maxStreamErrorBody bounds the non-SSE body read for an error message.
const maxStreamErrorBody = 64 << 10

// IsEventStream reports whether contentType is text/event-stream.
func IsEventStream(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "text/event-stream"
}

// StreamContentTypeError turns a 2xx streaming response that is not
// text/event-stream into an APIError that keeps the server's message and
// request id, instead of losing them in a bare truncated-stream error. The
// caller keeps ownership of resp and body.
func StreamContentTypeError(resp *http.Response, body []byte) error {
	var wire struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
			Param   string `json:"param"`
		} `json:"error"`
	}
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		RequestID: firstNonEmpty(
			resp.Header.Get("x-request-id"),
			resp.Header.Get("x-ds-trace-id"),
			resp.Header.Get("request-id"),
		),
		Body: body,
	}
	if json.Unmarshal(body, &wire) == nil && wire.Error != nil {
		apiErr.Message, apiErr.Type, apiErr.Code, apiErr.Param =
			wire.Error.Message, wire.Error.Type, wire.Error.Code, wire.Error.Param
	} else {
		apiErr.Message = strings.TrimSpace(string(body))
	}
	if apiErr.Message == "" {
		apiErr.Message = "response is not text/event-stream"
	}
	apiErr.Message = fmt.Sprintf("expected text/event-stream, got %q: %s", resp.Header.Get("Content-Type"), apiErr.Message)
	return apiErr
}
