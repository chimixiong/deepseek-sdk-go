package messages

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// BlockType identifies a content block in a request or response.
type BlockType string

// Content block types.
const (
	BlockText                BlockType = "text"
	BlockImage               BlockType = "image"
	BlockDocument            BlockType = "document"
	BlockToolResult          BlockType = "tool_result"
	BlockThinking            BlockType = "thinking"
	BlockToolUse             BlockType = "tool_use"
	BlockServerToolUse       BlockType = "server_tool_use"
	BlockWebSearchToolResult BlockType = "web_search_tool_result"
	BlockWebFetchToolResult  BlockType = "web_fetch_tool_result"
)

// Content is the string/block-array/null union used by message content,
// system prompts and tool results.
type Content struct {
	// Text is used when the content is a plain string.
	Text string
	// Blocks is used when the content is an array of blocks.
	Blocks []Block
	// Null marshals the content as JSON null.
	Null bool
}

// MarshalJSON encodes the union.
func (c Content) MarshalJSON() ([]byte, error) {
	switch {
	case c.Null:
		return []byte("null"), nil
	case c.Blocks != nil:
		return json.Marshal(c.Blocks)
	default:
		return json.Marshal(c.Text)
	}
}

// UnmarshalJSON decodes a string, an array of blocks or null.
func (c *Content) UnmarshalJSON(data []byte) error {
	*c = Content{}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("messages: empty content")
	}
	if bytes.Equal(trimmed, []byte("null")) {
		c.Null = true
		return nil
	}
	switch trimmed[0] {
	case '"':
		return json.Unmarshal(trimmed, &c.Text)
	case '[':
		return json.Unmarshal(trimmed, &c.Blocks)
	default:
		return fmt.Errorf("messages: content must be a string, an array of blocks or null")
	}
}

// Source is the base64/url/file/text source union used by image and document
// blocks.
type Source struct {
	Type      SourceType `json:"type"`
	MediaType string     `json:"media_type"`
	Data      string     `json:"data"`
	URL       string     `json:"url"`
	FileID    string     `json:"file_id"`
}

// SourceType identifies a Source variant.
type SourceType string

// Source variants.
const (
	SourceBase64 SourceType = "base64"
	SourceURL    SourceType = "url"
	SourceFile   SourceType = "file"
	SourceText   SourceType = "text"
)

// MarshalJSON encodes the source variant. Data is always emitted for base64
// and text sources, where the field is required even when empty.
func (s Source) MarshalJSON() ([]byte, error) {
	switch s.Type {
	case SourceBase64, SourceText:
		return json.Marshal(struct {
			Type      SourceType `json:"type"`
			MediaType string     `json:"media_type,omitempty"`
			Data      string     `json:"data"`
		}{s.Type, s.MediaType, s.Data})
	case SourceURL:
		return json.Marshal(struct {
			Type SourceType `json:"type"`
			URL  string     `json:"url"`
		}{s.Type, s.URL})
	case SourceFile:
		return json.Marshal(struct {
			Type   SourceType `json:"type"`
			FileID string     `json:"file_id"`
		}{s.Type, s.FileID})
	default:
		return json.Marshal(struct {
			Type      SourceType `json:"type"`
			MediaType string     `json:"media_type,omitempty"`
			Data      string     `json:"data,omitempty"`
			URL       string     `json:"url,omitempty"`
			FileID    string     `json:"file_id,omitempty"`
		}{s.Type, s.MediaType, s.Data, s.URL, s.FileID})
	}
}

// Citation is a web search result location attached to a text block.
type Citation struct {
	Type           CitationType `json:"type"`
	URL            string       `json:"url"`
	Title          *string      `json:"title,omitempty"`
	EncryptedIndex string       `json:"encrypted_index"`
	CitedText      string       `json:"cited_text,omitempty"`
}

// CitationType is the citation discriminator.
type CitationType string

// CitationWebSearchResult is the only citation type the API returns.
const CitationWebSearchResult CitationType = "web_search_result_location"

// Caller identifies who invoked a server tool.
type Caller struct {
	Type string `json:"type"`
}

// CallerDirect marks a direct server tool call.
const CallerDirect = "direct"

// WebToolKind is the DeepSeek extension marking which web capability produced
// a server tool block.
type WebToolKind string

// Web tool kinds.
const (
	KindSearch          WebToolKind = "search"
	KindDeepSearch      WebToolKind = "deep_search"
	KindReadLink        WebToolKind = "read_link"
	KindImageSearch     WebToolKind = "image_search"
	KindDeepImageSearch WebToolKind = "deep_image_search"
)

// WebSearchResult is an element of a web_search_tool_result content array:
// either a search result or a search error.
type WebSearchResult struct {
	Type             WebSearchResultType `json:"type"`
	Title            string              `json:"title"`
	URL              string              `json:"url"`
	EncryptedContent string              `json:"encrypted_content"`
	PageAge          *string             `json:"page_age"`
	Snippet          *string             `json:"snippet"`
	SiteName         *string             `json:"site_name"`
	SiteIcon         *string             `json:"site_icon"`
	ImageURL         *string             `json:"image_url"`
	ErrorCode        string              `json:"error_code"`
}

// WebSearchResultType is the item discriminator.
type WebSearchResultType string

// Web search result item types.
const (
	SearchResultWebSearch WebSearchResultType = "web_search_result"
	SearchResultError     WebSearchResultType = "web_search_tool_result_error"
)

// MarshalJSON encodes a result or an error item.
func (r WebSearchResult) MarshalJSON() ([]byte, error) {
	if r.Type == SearchResultError {
		return json.Marshal(struct {
			Type      WebSearchResultType `json:"type"`
			ErrorCode string              `json:"error_code"`
		}{r.Type, r.ErrorCode})
	}
	return json.Marshal(struct {
		Type             WebSearchResultType `json:"type"`
		Title            string              `json:"title"`
		URL              string              `json:"url"`
		EncryptedContent string              `json:"encrypted_content"`
		PageAge          *string             `json:"page_age,omitempty"`
		Snippet          *string             `json:"snippet,omitempty"`
		SiteName         *string             `json:"site_name,omitempty"`
		SiteIcon         *string             `json:"site_icon,omitempty"`
		ImageURL         *string             `json:"image_url,omitempty"`
	}{r.Type, r.Title, r.URL, r.EncryptedContent, r.PageAge, r.Snippet, r.SiteName, r.SiteIcon, r.ImageURL})
}

// WebFetchResult is the content object of a web_fetch_tool_result block: either
// a fetched page or a fetch error.
type WebFetchResult struct {
	Type        WebFetchResultType `json:"type"`
	URL         string             `json:"url,omitempty"`
	Content     *WebFetchDocument  `json:"content,omitempty"`
	PublishedAt *string            `json:"published_at,omitempty"`
	ErrorCode   string             `json:"error_code,omitempty"`
}

// WebFetchResultType is the content discriminator.
type WebFetchResultType string

// Web fetch result types.
const (
	WebFetchResultPage  WebFetchResultType = "web_fetch_result"
	WebFetchResultError WebFetchResultType = "web_fetch_tool_result_error"
)

// WebFetchDocument is the text document describing a fetched page. The API
// returns no body text; read_link pages travel in the block signature.
type WebFetchDocument struct {
	Type   string  `json:"type,omitempty"`
	Source *Source `json:"source,omitempty"`
	Title  *string `json:"title,omitempty"`
}

// Block is one request or response content block. The Type field selects which
// of the remaining fields apply.
type Block struct {
	Type BlockType

	// Text blocks.
	Text      string
	Citations []Citation

	// Image and document blocks.
	Source *Source
	Title  *string

	// Tool result blocks.
	ToolUseID string
	Content   *Content
	IsError   *bool

	// Thinking blocks. Signature is opaque and must be echoed back.
	Thinking  string
	Signature *string

	// Tool use blocks; server tool use also sets Caller and Kind.
	ID     string
	Name   string
	Input  json.RawMessage
	Caller *Caller

	// Kind marks DeepSeek web tool blocks.
	Kind WebToolKind

	// SearchResults is the content array of a web_search_tool_result block.
	SearchResults []WebSearchResult

	// Fetch is the content object of a web_fetch_tool_result block.
	Fetch *WebFetchResult

	// Raw preserves blocks whose type the SDK does not model.
	Raw json.RawMessage
}

// MarshalJSON encodes the block selected by Type.
func (b Block) MarshalJSON() ([]byte, error) {
	switch b.Type {
	case BlockText:
		return json.Marshal(struct {
			Type      BlockType  `json:"type"`
			Text      string     `json:"text"`
			Citations []Citation `json:"citations,omitempty"`
		}{b.Type, b.Text, b.Citations})
	case BlockImage:
		return json.Marshal(struct {
			Type   BlockType `json:"type"`
			Source *Source   `json:"source"`
		}{b.Type, b.Source})
	case BlockDocument:
		return json.Marshal(struct {
			Type   BlockType `json:"type"`
			Source *Source   `json:"source"`
			Title  *string   `json:"title,omitempty"`
		}{b.Type, b.Source, b.Title})
	case BlockToolResult:
		return json.Marshal(struct {
			Type      BlockType `json:"type"`
			ToolUseID string    `json:"tool_use_id"`
			Content   *Content  `json:"content,omitempty"`
			IsError   *bool     `json:"is_error,omitempty"`
		}{b.Type, b.ToolUseID, b.Content, b.IsError})
	case BlockThinking:
		return json.Marshal(struct {
			Type      BlockType `json:"type"`
			Thinking  string    `json:"thinking"`
			Signature *string   `json:"signature,omitempty"`
		}{b.Type, b.Thinking, b.Signature})
	case BlockToolUse:
		return json.Marshal(struct {
			Type  BlockType       `json:"type"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}{b.Type, b.ID, b.Name, b.inputOrDefault()})
	case BlockServerToolUse:
		return json.Marshal(struct {
			Type   BlockType       `json:"type"`
			ID     string          `json:"id"`
			Name   string          `json:"name"`
			Input  json.RawMessage `json:"input"`
			Caller *Caller         `json:"caller,omitempty"`
			Kind   WebToolKind     `json:"kind,omitempty"`
		}{b.Type, b.ID, b.Name, b.inputOrDefault(), b.Caller, b.Kind})
	case BlockWebSearchToolResult:
		results := b.SearchResults
		if results == nil {
			results = []WebSearchResult{}
		}
		return json.Marshal(struct {
			Type      BlockType         `json:"type"`
			ToolUseID string            `json:"tool_use_id"`
			Content   []WebSearchResult `json:"content"`
			Kind      WebToolKind       `json:"kind,omitempty"`
		}{b.Type, b.ToolUseID, results, b.Kind})
	case BlockWebFetchToolResult:
		return json.Marshal(struct {
			Type      BlockType       `json:"type"`
			ToolUseID string          `json:"tool_use_id"`
			Content   *WebFetchResult `json:"content"`
			Kind      WebToolKind     `json:"kind,omitempty"`
			Signature *string         `json:"signature,omitempty"`
		}{b.Type, b.ToolUseID, b.Fetch, b.Kind, b.Signature})
	default:
		if len(b.Raw) > 0 {
			return b.Raw, nil
		}
		if b.Type == "" {
			return nil, fmt.Errorf("messages: content block without a type")
		}
		return json.Marshal(struct {
			Type BlockType `json:"type"`
		}{b.Type})
	}
}

// UnmarshalJSON decodes a block by its type. Unknown types are preserved in Raw.
func (b *Block) UnmarshalJSON(data []byte) error {
	*b = Block{}
	var head struct {
		Type BlockType `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return fmt.Errorf("messages: decode content block: %w", err)
	}
	b.Type = head.Type
	switch head.Type {
	case BlockText:
		var wire struct {
			Text      string     `json:"text"`
			Citations []Citation `json:"citations"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return err
		}
		b.Text, b.Citations = wire.Text, wire.Citations
	case BlockImage, BlockDocument:
		var wire struct {
			Source *Source `json:"source"`
			Title  *string `json:"title"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return err
		}
		b.Source, b.Title = wire.Source, wire.Title
	case BlockToolResult:
		var wire struct {
			ToolUseID string   `json:"tool_use_id"`
			Content   *Content `json:"content"`
			IsError   *bool    `json:"is_error"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return err
		}
		b.ToolUseID, b.Content, b.IsError = wire.ToolUseID, wire.Content, wire.IsError
	case BlockThinking:
		var wire struct {
			Thinking  string  `json:"thinking"`
			Signature *string `json:"signature"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return err
		}
		b.Thinking, b.Signature = wire.Thinking, wire.Signature
	case BlockToolUse:
		var wire struct {
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return err
		}
		b.ID, b.Name, b.Input = wire.ID, wire.Name, wire.Input
	case BlockServerToolUse:
		var wire struct {
			ID     string          `json:"id"`
			Name   string          `json:"name"`
			Input  json.RawMessage `json:"input"`
			Caller *Caller         `json:"caller"`
			Kind   WebToolKind     `json:"kind"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return err
		}
		b.ID, b.Name, b.Input, b.Caller, b.Kind = wire.ID, wire.Name, wire.Input, wire.Caller, wire.Kind
	case BlockWebSearchToolResult:
		var wire struct {
			ToolUseID string            `json:"tool_use_id"`
			Content   []WebSearchResult `json:"content"`
			Kind      WebToolKind       `json:"kind"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return err
		}
		b.ToolUseID, b.SearchResults, b.Kind = wire.ToolUseID, wire.Content, wire.Kind
	case BlockWebFetchToolResult:
		var wire struct {
			ToolUseID string          `json:"tool_use_id"`
			Content   *WebFetchResult `json:"content"`
			Kind      WebToolKind     `json:"kind"`
			Signature *string         `json:"signature"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			return err
		}
		b.ToolUseID, b.Fetch, b.Kind, b.Signature = wire.ToolUseID, wire.Content, wire.Kind, wire.Signature
	default:
		b.Raw = append(json.RawMessage(nil), data...)
	}
	return nil
}

func (b Block) inputOrDefault() json.RawMessage {
	if len(b.Input) > 0 {
		return b.Input
	}
	return json.RawMessage("{}")
}
