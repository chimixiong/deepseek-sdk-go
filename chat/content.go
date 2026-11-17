package chat

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/chimixiong/deepseek-sdk-go"
)

// Content is a string or a block array. The zero value marshals as an empty
// string, which is what assistant tool-call turns send; NullContent marshals
// as JSON null.
type Content struct {
	Text  string
	Parts []ContentPart
	Null  bool
}

// TextContent builds a plain string content.
func TextContent(text string) Content { return Content{Text: text} }

// PartsContent builds a block-array content.
func PartsContent(parts ...ContentPart) Content { return Content{Parts: parts} }

// NullContent builds a JSON null content.
func NullContent() Content { return Content{Null: true} }

// MarshalJSON implements the string|block[]|null union.
func (c Content) MarshalJSON() ([]byte, error) {
	if c.Null {
		return []byte("null"), nil
	}
	if c.Parts != nil {
		return json.Marshal(c.Parts)
	}
	return json.Marshal(c.Text)
}

// UnmarshalJSON accepts a string, a block array or null.
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
	c.Text = ""
	c.Parts = parts
	c.Null = false
	return nil
}

// PartType is a content block discriminator.
type PartType string

const (
	PartText  PartType = "text"
	PartImage PartType = "image_url"
	PartFile  PartType = "file"
)

// ContentPart is a text, image or file block. Unknown types are preserved in
// Raw and marshalled back verbatim; unmodelled fields of known types are kept
// in Extra and merged back on marshal.
type ContentPart struct {
	Type     PartType
	Text     string
	ImageURL *ImageURL
	File     *FileRef
	Raw      json.RawMessage
	Extra    map[string]json.RawMessage
}

// ImageURL points at an image by URL or data URL.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// FileRef references an uploaded file or carries inline file data.
type FileRef struct {
	FileID   string `json:"file_id,omitempty"`
	FileData string `json:"file_data,omitempty"`
	Filename string `json:"filename,omitempty"`
}

var errEmptyPartType = errors.New("deepseek/chat: content part has no type")

// TextPart builds a text block.
func TextPart(text string) ContentPart { return ContentPart{Type: PartText, Text: text} }

// ImagePart builds an image_url block.
func ImagePart(url, detail string) ContentPart {
	return ContentPart{Type: PartImage, ImageURL: &ImageURL{URL: url, Detail: detail}}
}

// FilePart builds a file block.
func FilePart(ref FileRef) ContentPart { return ContentPart{Type: PartFile, File: &ref} }

type contentPartWire struct {
	Type     PartType  `json:"type"`
	Text     *string   `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
	File     *FileRef  `json:"file,omitempty"`
}

// MarshalJSON encodes the block by type, replays an unknown block or injects
// Extra. Text parts always carry text, even when empty.
func (p ContentPart) MarshalJSON() ([]byte, error) {
	if p.Raw != nil {
		return p.Raw, nil
	}
	if p.Type == "" {
		return nil, errEmptyPartType
	}
	wire := contentPartWire{
		Type:     p.Type,
		ImageURL: p.ImageURL,
		File:     p.File,
	}
	if p.Type == PartText || p.Text != "" {
		text := p.Text
		wire.Text = &text
	}
	return deepseek.MarshalWithExtra(wire, p.Extra)
}

// UnmarshalJSON decodes a known block or keeps the raw JSON, capturing
// unmodelled fields in Extra for known types.
func (p *ContentPart) UnmarshalJSON(data []byte) error {
	var wire contentPartWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	text := ""
	if wire.Text != nil {
		text = *wire.Text
	}
	*p = ContentPart{Type: wire.Type, Text: text, ImageURL: wire.ImageURL, File: wire.File}
	switch wire.Type {
	case PartText, PartImage, PartFile:
		extra, err := splitContentPartExtra(data)
		if err != nil {
			return err
		}
		p.Extra = extra
		return nil
	default:
		p.Raw = append(json.RawMessage(nil), data...)
		return nil
	}
}

// splitContentPartExtra returns the object fields not modelled by
// contentPartWire.
func splitContentPartExtra(data []byte) (map[string]json.RawMessage, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	t := reflect.TypeOf(contentPartWire{})
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
