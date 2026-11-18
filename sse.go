package deepseek

import (
	"bufio"
	"fmt"
	"io"
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
