package responses

import (
	"encoding/json"
	"errors"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Content is a message content or tool output: a string or a content-part
// array. The zero value marshals as an empty string; Null marshals as JSON
// null.
type Content struct {
	Text  string
	Parts []ContentPart
	Null  bool
}

// TextContent builds a plain string content.
func TextContent(text string) Content { return Content{Text: text} }

// PartsContent builds a content-part array.
func PartsContent(parts ...ContentPart) Content { return Content{Parts: parts} }

// NullContent builds a JSON null content.
func NullContent() Content { return Content{Null: true} }

// MarshalJSON implements the string|part[]|null union.
func (c Content) MarshalJSON() ([]byte, error) {
	if c.Null {
		return []byte("null"), nil
	}
	if c.Parts != nil {
		return json.Marshal(c.Parts)
	}
	return json.Marshal(c.Text)
}

// UnmarshalJSON accepts a string, a part array or null.
func (c *Content) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*c = NullContent()
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*c = TextContent(text)
		return nil
	}
	var parts []ContentPart
	if err := json.Unmarshal(data, &parts); err != nil {
		return err
	}
	*c = Content{Parts: parts}
	return nil
}

// ContentPartType is the content-part discriminator.
type ContentPartType string

const (
	PartInputText     ContentPartType = "input_text"
	PartOutputText    ContentPartType = "output_text"
	PartInputImage    ContentPartType = "input_image"
	PartInputFile     ContentPartType = "input_file"
	PartReasoningText ContentPartType = "reasoning_text"
)

// ImageDetail controls image processing. Setting FileID ignores it.
type ImageDetail string

const (
	DetailLow      ImageDetail = "low"
	DetailHigh     ImageDetail = "high"
	DetailOriginal ImageDetail = "original"
	DetailAuto     ImageDetail = "auto"
)

// ContentPart is an input_text, output_text, input_image, input_file or
// reasoning_text part. Unknown part types are preserved verbatim in Raw.
type ContentPart struct {
	Type ContentPartType
	// Text is set for text and reasoning parts.
	Text string
	// ImageURL is an http(s) URL or data URL; mutually exclusive with FileID.
	ImageURL string
	// FileID references a file uploaded through the Files API.
	FileID string
	// Detail applies to input_image.
	Detail ImageDetail
	// Annotations and Logprobs are returned on output_text parts.
	Annotations []json.RawMessage
	Logprobs    []json.RawMessage
	// Raw replays parts whose type this release does not model.
	Raw json.RawMessage
	// Extra merges additional top-level fields into the part object.
	Extra map[string]json.RawMessage `json:"-"`
}

// InputTextPart builds an input_text part.
func InputTextPart(text string) ContentPart {
	return ContentPart{Type: PartInputText, Text: text}
}

// OutputTextPart builds an output_text part.
func OutputTextPart(text string) ContentPart {
	return ContentPart{Type: PartOutputText, Text: text}
}

// InputImagePart builds an input_image part from a URL or data URL.
func InputImagePart(imageURL string, detail ImageDetail) ContentPart {
	return ContentPart{Type: PartInputImage, ImageURL: imageURL, Detail: detail}
}

// InputImageFilePart builds an input_image part from an uploaded file ID.
func InputImageFilePart(fileID string, detail ImageDetail) ContentPart {
	return ContentPart{Type: PartInputImage, FileID: fileID, Detail: detail}
}

// InputFilePart builds an input_file part from an uploaded file ID.
func InputFilePart(fileID string) ContentPart {
	return ContentPart{Type: PartInputFile, FileID: fileID}
}

// ReasoningTextPart builds a reasoning_text part.
func ReasoningTextPart(text string) ContentPart {
	return ContentPart{Type: PartReasoningText, Text: text}
}

type contentPartWire struct {
	Type        ContentPartType   `json:"type"`
	Text        *string           `json:"text,omitempty"`
	ImageURL    string            `json:"image_url,omitempty"`
	FileID      string            `json:"file_id,omitempty"`
	Detail      ImageDetail       `json:"detail,omitempty"`
	Annotations []json.RawMessage `json:"annotations,omitempty"`
	Logprobs    []json.RawMessage `json:"logprobs,omitempty"`
}

// MarshalJSON encodes the part by type, replays an unknown part or injects
// Extra.
func (p ContentPart) MarshalJSON() ([]byte, error) {
	if p.Raw != nil {
		return p.Raw, nil
	}
	if p.Type == "" {
		return nil, errors.New("deepseek/responses: content part has no type")
	}
	wire := contentPartWire{
		Type:        p.Type,
		ImageURL:    p.ImageURL,
		FileID:      p.FileID,
		Detail:      p.Detail,
		Annotations: p.Annotations,
		Logprobs:    p.Logprobs,
	}
	switch p.Type {
	case PartInputText, PartOutputText, PartReasoningText:
		text := p.Text
		wire.Text = &text
	}
	return deepseek.MarshalWithExtra(wire, p.Extra)
}

// UnmarshalJSON decodes a known part, keeps an unknown one in Raw and captures
// unmodelled fields in Extra.
func (p *ContentPart) UnmarshalJSON(data []byte) error {
	var wire contentPartWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*p = ContentPart{
		Type:        wire.Type,
		ImageURL:    wire.ImageURL,
		FileID:      wire.FileID,
		Detail:      wire.Detail,
		Annotations: wire.Annotations,
		Logprobs:    wire.Logprobs,
	}
	if wire.Text != nil {
		p.Text = *wire.Text
	}
	switch wire.Type {
	case PartInputText, PartOutputText, PartInputImage, PartInputFile, PartReasoningText:
	default:
		p.Raw = append(json.RawMessage(nil), data...)
		return nil
	}
	extra, err := splitExtra(data, contentPartWire{})
	if err != nil {
		return err
	}
	p.Extra = extra
	return nil
}
