// Package history converts conversation history between the Chat Completions,
// Responses and Messages protocols.
//
// The neutral model deliberately has no thinking field. DeepSeek requires each
// protocol's reasoning payload to be replayed verbatim (Messages thinking
// blocks carry a signature, Responses reasoning items carry server ids), so
// reasoning cannot be translated. Conversions therefore drop reasoning and
// report a Warning; callers decide whether that loss is acceptable.
package history

import (
	"encoding/json"
	"fmt"
)

// Role is a neutral speaker.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Transcript is a system prompt plus ordered turns. System stays separate
// because the three protocols carry it differently: a system message, the
// top-level instructions field, or the top-level system field.
type Transcript struct {
	System string
	Turns  []Turn
}

// Turn is one user or assistant turn. Consecutive turns of the same role are
// merged by the protocol converters.
type Turn struct {
	Role  Role
	Parts []Part
}

// Part is one piece of a turn. Exactly one pointer is set.
type Part struct {
	Text       *string
	Image      *Image
	File       *File
	ToolCall   *ToolCall
	ToolResult *ToolResult
}

// Image is an image reference. Set either URL or Data with MediaType; FileID
// refers to an image uploaded through the Files API.
type Image struct {
	URL       string
	Data      []byte
	MediaType string
	FileID    string
	Detail    string
}

// File is a document reference: an uploaded file id or inline bytes. PDFs,
// Word and PowerPoint documents cross protocols as File, images as Image.
type File struct {
	FileID    string
	Data      []byte
	MediaType string
	Filename  string
}

// ToolCall is a model-requested function call. Arguments is raw JSON text.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

// ToolResult is the output of a call. Its parts may include images.
type ToolResult struct {
	CallID string
	Parts  []Part
}

// Warning reports information lost or repaired during conversion.
type Warning struct {
	// Turn indexes Transcript.Turns; -1 means the system prompt.
	Turn int
	// Field is the affected field, for example "reasoning_content".
	Field string
	// Reason explains what happened.
	Reason string
}

func (w Warning) String() string {
	return fmt.Sprintf("turn %d %s: %s", w.Turn, w.Field, w.Reason)
}

// Options tunes conversions.
type Options struct {
	// DropImages removes images instead of translating them. Use it when the
	// target model has no vision input (for example deepseek-v4-pro).
	DropImages bool
	// DropToolResults removes tool results whose call has no matching tool call
	// in the preceding assistant turn. Results with a matching call are always
	// kept. Without the option an unpaired result is converted and reported
	// with a warning.
	DropToolResults bool
}

// Text builds a text part.
func Text(text string) Part { return Part{Text: new(text)} }

// ImagePart builds an image part.
func ImagePart(image Image) Part { return Part{Image: &image} }

// FilePart builds a document part.
func FilePart(file File) Part { return Part{File: &file} }

// Call builds a tool-call part.
func Call(id, name string, arguments json.RawMessage) Part {
	return Part{ToolCall: &ToolCall{ID: id, Name: name, Arguments: arguments}}
}

// Result builds a tool-result part.
func Result(callID string, parts ...Part) Part {
	return Part{ToolResult: &ToolResult{CallID: callID, Parts: parts}}
}

// Texts returns the concatenated text of parts, ignoring non-text parts.
func Texts(parts []Part) string {
	var out string
	for _, part := range parts {
		if part.Text != nil {
			if out != "" {
				out += "\n"
			}
			out += *part.Text
		}
	}
	return out
}

// HasToolResult reports whether parts contain a tool result.
func HasToolResult(parts []Part) bool {
	for _, part := range parts {
		if part.ToolResult != nil {
			return true
		}
	}
	return false
}

func warning(turn int, field, reason string) Warning {
	return Warning{Turn: turn, Field: field, Reason: reason}
}

func unpairedResultReason(callID string) string {
	return fmt.Sprintf("no matching tool call %q", callID)
}

// imageHasContent reports whether an image carries a reference that can be
// converted without fabricating an empty URL.
func imageHasContent(image Image) bool {
	return image.URL != "" || image.FileID != "" || len(image.Data) > 0
}
