package responses

import (
	"encoding/json"

	"github.com/chimixiong/deepseek-sdk-go"
)

// ItemType is the input or output item discriminator.
type ItemType string

const (
	ItemMessage              ItemType = "message"
	ItemFunctionCall         ItemType = "function_call"
	ItemFunctionCallOutput   ItemType = "function_call_output"
	ItemCustomToolCall       ItemType = "custom_tool_call"
	ItemCustomToolCallOutput ItemType = "custom_tool_call_output"
	ItemReasoning            ItemType = "reasoning"
	ItemWebSearchCall        ItemType = "web_search_call"
	ItemToolSearchCall       ItemType = "tool_search_call"
)

// Role is a message author. "developer" is treated as "system" on
// deepseek-flash and as "user" on deepseek-v4-pro.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleDeveloper Role = "developer"
)

// ItemStatus is an output item status. "failed" is used by failed
// web_search_call items.
type ItemStatus string

const (
	ItemInProgress ItemStatus = "in_progress"
	ItemCompleted  ItemStatus = "completed"
	ItemIncomplete ItemStatus = "incomplete"
	ItemFailed     ItemStatus = "failed"
)

// WebToolKind labels a server-side web tool result. Values must be echoed back
// unchanged in multi-turn conversations.
type WebToolKind string

const (
	KindSearch          WebToolKind = "search"
	KindDeepSearch      WebToolKind = "deep_search"
	KindReadLink        WebToolKind = "read_link"
	KindImageSearch     WebToolKind = "image_search"
	KindDeepImageSearch WebToolKind = "deep_image_search"
)

// WebSearchErrorCode is the failure code of a web_search_call item.
type WebSearchErrorCode string

const (
	WebErrorUnavailable      WebSearchErrorCode = "unavailable"
	WebErrorURLNotAccessible WebSearchErrorCode = "url_not_accessible"
	WebErrorURLNotAllowed    WebSearchErrorCode = "url_not_allowed"
)

// InputItem is one Responses API input item. Type may be omitted for message
// items that carry a role. Fields that do not apply to the item type stay
// empty.
type InputItem struct {
	Type ItemType `json:"type,omitempty"`
	Role Role     `json:"role,omitempty"`
	// Content is the message text or parts; for reasoning items it holds
	// reasoning_text parts.
	Content *Content `json:"content,omitempty"`
	CallID  string   `json:"call_id,omitempty"`
	Name    string   `json:"name,omitempty"`
	// Arguments is the JSON argument string of a function_call.
	Arguments string `json:"arguments,omitempty"`
	// Input is the custom tool input of a custom_tool_call item.
	Input *string `json:"input,omitempty"`
	// Output is the tool result of a function_call_output or
	// custom_tool_call_output item.
	Output *Content `json:"output,omitempty"`

	// The remaining fields echo a previous web_search_call item back to the
	// service.
	ID        string           `json:"id,omitempty"`
	Status    ItemStatus       `json:"status,omitempty"`
	Kind      WebToolKind      `json:"kind,omitempty"`
	Action    *WebSearchAction `json:"action,omitempty"`
	Page      *Page            `json:"page,omitempty"`
	Signature string           `json:"signature,omitempty"`
	Error     *WebSearchError  `json:"error,omitempty"`
	// Extra merges additional top-level fields into the item object.
	Extra map[string]json.RawMessage `json:"-"`
}

type inputItemAlias InputItem

// MarshalJSON injects Extra.
func (it InputItem) MarshalJSON() ([]byte, error) {
	return deepseek.MarshalWithExtra(inputItemAlias(it), it.Extra)
}

// UnmarshalJSON keeps fields this release does not model.
func (it *InputItem) UnmarshalJSON(data []byte) error {
	var wire inputItemAlias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*it = InputItem(wire)
	extra, err := splitExtra(data, InputItem{})
	if err != nil {
		return err
	}
	it.Extra = extra
	return nil
}

// OutputItem is one item produced by the model.
type OutputItem struct {
	Type    ItemType      `json:"type,omitempty"`
	ID      string        `json:"id,omitempty"`
	Status  ItemStatus    `json:"status,omitempty"`
	Role    Role          `json:"role,omitempty"`
	Content []ContentPart `json:"content,omitempty"`
	// Phase is "final_answer" for final assistant messages.
	Phase     string           `json:"phase,omitempty"`
	CallID    string           `json:"call_id,omitempty"`
	Name      string           `json:"name,omitempty"`
	Arguments string           `json:"arguments,omitempty"`
	Input     *string          `json:"input,omitempty"`
	Kind      WebToolKind      `json:"kind,omitempty"`
	Action    *WebSearchAction `json:"action,omitempty"`
	Page      *Page            `json:"page,omitempty"`
	Signature string           `json:"signature,omitempty"`
	Error     *WebSearchError  `json:"error,omitempty"`
	// Extra merges additional top-level fields into the item object.
	Extra map[string]json.RawMessage `json:"-"`
}

type outputItemAlias OutputItem

// MarshalJSON injects Extra.
func (it OutputItem) MarshalJSON() ([]byte, error) {
	return deepseek.MarshalWithExtra(outputItemAlias(it), it.Extra)
}

// UnmarshalJSON keeps fields this release does not model.
func (it *OutputItem) UnmarshalJSON(data []byte) error {
	var wire outputItemAlias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*it = OutputItem(wire)
	extra, err := splitExtra(data, OutputItem{})
	if err != nil {
		return err
	}
	it.Extra = extra
	return nil
}

// WebSearchActionType is the action discriminator.
type WebSearchActionType string

const (
	ActionSearch     WebSearchActionType = "search"
	ActionOpenPage   WebSearchActionType = "open_page"
	ActionFindInPage WebSearchActionType = "find_in_page"
)

// WebSearchAction describes one server-side web operation: a search, an
// opened page or a find-in-page query.
type WebSearchAction struct {
	Type    WebSearchActionType `json:"type,omitempty"`
	Queries []string            `json:"queries,omitempty"`
	Sources []SearchSource      `json:"sources,omitempty"`
	URL     string              `json:"url,omitempty"`
	Pattern string              `json:"pattern,omitempty"`
	// Extra merges additional top-level fields into the action object.
	Extra map[string]json.RawMessage `json:"-"`
}

type webSearchActionAlias WebSearchAction

// MarshalJSON injects Extra.
func (a WebSearchAction) MarshalJSON() ([]byte, error) {
	return deepseek.MarshalWithExtra(webSearchActionAlias(a), a.Extra)
}

// UnmarshalJSON keeps fields this release does not model.
func (a *WebSearchAction) UnmarshalJSON(data []byte) error {
	var wire webSearchActionAlias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*a = WebSearchAction(wire)
	extra, err := splitExtra(data, WebSearchAction{})
	if err != nil {
		return err
	}
	a.Extra = extra
	return nil
}

// SearchSource is one search result attached to an action.
type SearchSource struct {
	Type        string `json:"type,omitempty"`
	URL         string `json:"url,omitempty"`
	Title       string `json:"title,omitempty"`
	Snippet     string `json:"snippet,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	SiteIcon    string `json:"site_icon,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	// Extra merges additional top-level fields into the source object.
	Extra map[string]json.RawMessage `json:"-"`
}

type searchSourceAlias SearchSource

// MarshalJSON injects Extra.
func (s SearchSource) MarshalJSON() ([]byte, error) {
	return deepseek.MarshalWithExtra(searchSourceAlias(s), s.Extra)
}

// UnmarshalJSON keeps fields this release does not model.
func (s *SearchSource) UnmarshalJSON(data []byte) error {
	var wire searchSourceAlias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*s = SearchSource(wire)
	extra, err := splitExtra(data, SearchSource{})
	if err != nil {
		return err
	}
	s.Extra = extra
	return nil
}

// Page is an opened or read page. Its body is not returned; a successful
// read_link call carries an encrypted Signature instead.
type Page struct {
	URL         string `json:"url,omitempty"`
	Title       string `json:"title,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

// WebSearchError describes a failed web operation.
type WebSearchError struct {
	Code    WebSearchErrorCode `json:"code,omitempty"`
	Message string             `json:"message,omitempty"`
}
