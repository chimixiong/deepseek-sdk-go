package messages

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	deepseek "github.com/chimixiong/deepseek-sdk-go"
)

// Role identifies the author of a message.
type Role string

// Message roles.
const (
	RoleUser           Role = "user"
	RoleAssistant      Role = "assistant"
	RoleLatestReminder Role = "latest_reminder"
)

// Request is a CreateMessage request body.
type Request struct {
	// Model is a DeepSeek model id or a Claude model alias.
	Model string `json:"model"`
	// Messages is the conversation history; at least one message is required.
	Messages []Message `json:"messages"`
	// System is a string, an array of text blocks or null.
	System *Content `json:"system,omitempty"`
	// MaxTokens bounds the generated tokens, thinking included.
	MaxTokens *int `json:"max_tokens,omitempty"`
	// Stream requests server-sent events. CreateStream sets it automatically
	// and Create rejects it.
	Stream bool `json:"stream,omitempty"`
	// Thinking selects the thinking mode. When nil, output_config.effort or
	// the model default decides.
	Thinking *Thinking `json:"thinking,omitempty"`
	// OutputConfig carries the reasoning effort.
	OutputConfig *OutputConfig `json:"output_config,omitempty"`
	// Tools lists client and server tools. Names must be unique.
	Tools []Tool `json:"tools,omitempty"`
	// ToolChoice controls tool selection. A nil pointer omits the field; use
	// &ToolChoice{Null: true} to send null.
	ToolChoice *ToolChoice `json:"tool_choice,omitempty"`
	// StopSequences holds up to 16 non-empty stop strings.
	StopSequences []string `json:"stop_sequences,omitempty"`
	// Temperature ranges from 0 to 2 and is fixed to 1 in thinking mode.
	Temperature *float64 `json:"temperature,omitempty"`
	// TopP must be greater than 0 and at most 1.
	TopP *float64 `json:"top_p,omitempty"`
	// Metadata carries the end-user id.
	Metadata *Metadata `json:"metadata,omitempty"`
	// SafetyCheck is a DeepSeek extension enabling content moderation.
	SafetyCheck *SafetyCheck `json:"safety_check,omitempty"`
	// Title is a DeepSeek extension returning a conversation title.
	Title *Title `json:"title,omitempty"`
	// Extra carries fields the SDK does not model yet.
	Extra map[string]json.RawMessage `json:"-"`
}

// MarshalJSON encodes the request and merges Extra.
func (r *Request) MarshalJSON() ([]byte, error) {
	type plain Request
	return deepseek.MarshalWithExtra((*plain)(r), r.Extra)
}

// Message is one entry in Request.Messages.
type Message struct {
	Role    Role    `json:"role"`
	Content Content `json:"content"`
}

// ThinkingType selects the thinking mode.
type ThinkingType string

// Thinking modes.
const (
	ThinkingEnabled  ThinkingType = "enabled"
	ThinkingDisabled ThinkingType = "disabled"
	ThinkingAdaptive ThinkingType = "adaptive"
)

// Thinking configures reasoning. An explicit type wins over
// output_config.effort.
type Thinking struct {
	Type ThinkingType `json:"type"`
	// BudgetTokens is accepted for compatibility but ignored by the API.
	BudgetTokens *int `json:"budget_tokens,omitempty"`
}

// Effort is the reasoning effort.
type Effort string

// Reasoning effort levels. The API maps minimal to low, medium and xhigh to
// high, and ultra to max.
const (
	EffortNone    Effort = "none"
	EffortMinimal Effort = "minimal"
	EffortLow     Effort = "low"
	EffortMedium  Effort = "medium"
	EffortHigh    Effort = "high"
	EffortXHigh   Effort = "xhigh"
	EffortUltra   Effort = "ultra"
	EffortMax     Effort = "max"
)

// OutputConfig controls reasoning effort.
type OutputConfig struct {
	Effort Effort `json:"effort,omitempty"`
}

// ToolType selects a server tool; client tools leave it empty.
type ToolType string

// Server tool types.
const (
	ToolWebSearch20250305 ToolType = "web_search_20250305"
	ToolWebSearch20260209 ToolType = "web_search_20260209"
	ToolDeepSeekWebSearch ToolType = "deepseek_web_search"
	ToolDeepSeekReadURL   ToolType = "deepseek_read_url"
)

// Tool is a client or server tool declaration.
type Tool struct {
	// Type is empty for client tools; setting it selects a server tool.
	Type ToolType `json:"type,omitempty"`
	// Name is required for client tools and for web_search; the DeepSeek tools
	// leave it empty or repeat the type.
	Name string `json:"name,omitempty"`
	// Description explains a client tool.
	Description *string `json:"description,omitempty"`
	// InputSchema is the JSON Schema of a client tool; its top-level type must
	// be "object".
	InputSchema map[string]any `json:"input_schema,omitempty"`
	// MaxUses bounds web_search calls for this request.
	MaxUses *int `json:"max_uses,omitempty"`
}

// ToolChoiceType selects a tool choice variant.
type ToolChoiceType string

// Tool choice variants.
const (
	ToolChoiceAuto ToolChoiceType = "auto"
	ToolChoiceAny  ToolChoiceType = "any"
	ToolChoiceTool ToolChoiceType = "tool"
	ToolChoiceNone ToolChoiceType = "none"
)

// ToolChoice controls which tools the model may call. Null marshals as JSON
// null and is the default.
type ToolChoice struct {
	Type ToolChoiceType
	Name string
	Null bool
}

// MarshalJSON encodes the tool choice.
func (t ToolChoice) MarshalJSON() ([]byte, error) {
	if t.Null {
		return []byte("null"), nil
	}
	return json.Marshal(struct {
		Type ToolChoiceType `json:"type"`
		Name string         `json:"name,omitempty"`
	}{t.Type, t.Name})
}

// UnmarshalJSON decodes a tool choice object or null.
func (t *ToolChoice) UnmarshalJSON(data []byte) error {
	*t = ToolChoice{}
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		t.Null = true
		return nil
	}
	var wire struct {
		Type ToolChoiceType `json:"type"`
		Name string         `json:"name"`
	}
	if err := json.Unmarshal(trimmed, &wire); err != nil {
		return err
	}
	t.Type, t.Name = wire.Type, wire.Name
	return nil
}

// Metadata carries the end-user identifier.
type Metadata struct {
	UserID *string `json:"user_id,omitempty"`
}

// SafetyLevel is the DeepSeek content moderation level.
type SafetyLevel string

// Safety levels.
const (
	SafetyLow  SafetyLevel = "low"
	SafetyHigh SafetyLevel = "high"
)

// SafetyCheck enables DeepSeek content moderation; high is the only level
// that turns it on.
type SafetyCheck struct {
	Level SafetyLevel `json:"level"`
}

// Title asks the API to return a conversation title.
type Title struct {
	Enabled bool `json:"enabled"`
}

var toolNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Validate checks the documented request constraints. Semantic choices that
// depend on server-side model behavior, such as a named tool_choice requiring
// a non-thinking mode, are left to the server.
func (r *Request) Validate() error {
	if r == nil {
		return deepseek.Invalid("request", "must not be nil")
	}
	if r.Model == "" {
		return deepseek.Invalid("model", "is required")
	}
	if len(r.Messages) == 0 {
		return deepseek.Invalid("messages", "at least one message is required")
	}
	if r.MaxTokens != nil && *r.MaxTokens < 1 {
		return deepseek.Invalid("max_tokens", "must be at least 1, got %d", *r.MaxTokens)
	}
	if r.Temperature != nil && (*r.Temperature < 0 || *r.Temperature > 2) {
		return deepseek.Invalid("temperature", "must be between 0 and 2, got %v", *r.Temperature)
	}
	if r.TopP != nil && (*r.TopP <= 0 || *r.TopP > 1) {
		return deepseek.Invalid("top_p", "must be greater than 0 and at most 1, got %v", *r.TopP)
	}
	if len(r.StopSequences) > 16 {
		return deepseek.Invalid("stop_sequences", "must contain at most 16 entries, got %d", len(r.StopSequences))
	}
	for i, stop := range r.StopSequences {
		if stop == "" {
			return deepseek.Invalid(fmt.Sprintf("stop_sequences[%d]", i), "must not be empty")
		}
		if len(stop) > 512 {
			return deepseek.Invalid(fmt.Sprintf("stop_sequences[%d]", i), "must not exceed 512 bytes, got %d", len(stop))
		}
	}
	if r.Metadata != nil && r.Metadata.UserID != nil && len(*r.Metadata.UserID) > 512 {
		return deepseek.Invalid("metadata.user_id", "must not exceed 512 bytes, got %d", len(*r.Metadata.UserID))
	}
	if r.System != nil {
		if err := validateSystem(*r.System); err != nil {
			return err
		}
	}
	if r.Thinking != nil {
		if r.Thinking.Type == "" {
			return deepseek.Invalid("thinking.type", "is required")
		}
		switch r.Thinking.Type {
		case ThinkingEnabled, ThinkingDisabled, ThinkingAdaptive:
		default:
			return deepseek.Invalid("thinking.type", "must be %q, %q or %q, got %q", ThinkingEnabled, ThinkingDisabled, ThinkingAdaptive, r.Thinking.Type)
		}
		if r.Thinking.BudgetTokens != nil && *r.Thinking.BudgetTokens < 0 {
			return deepseek.Invalid("thinking.budget_tokens", "must not be negative, got %d", *r.Thinking.BudgetTokens)
		}
	}
	if r.OutputConfig != nil {
		switch r.OutputConfig.Effort {
		case "", EffortNone, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortUltra, EffortMax:
		default:
			return deepseek.Invalid("output_config.effort", "unsupported value %q", r.OutputConfig.Effort)
		}
	}
	if r.SafetyCheck != nil {
		switch r.SafetyCheck.Level {
		case SafetyLow, SafetyHigh:
		default:
			return deepseek.Invalid("safety_check.level", "must be %q or %q, got %q", SafetyLow, SafetyHigh, r.SafetyCheck.Level)
		}
	}
	if err := validateTools(r.Tools); err != nil {
		return err
	}
	if r.ToolChoice != nil {
		if err := validateToolChoice(*r.ToolChoice); err != nil {
			return err
		}
	}
	return validateMessages(r.Messages)
}

func validateSystem(system Content) error {
	if system.Null {
		return nil
	}
	for i, block := range system.Blocks {
		if block.Type != BlockText {
			return deepseek.Invalid(fmt.Sprintf("system[%d].type", i), "system blocks must be text, got %q", block.Type)
		}
	}
	return nil
}

func validateTools(tools []Tool) error {
	seen := make(map[string]bool, len(tools))
	for i, tool := range tools {
		field := fmt.Sprintf("tools[%d]", i)
		if tool.Name != "" {
			if seen[tool.Name] {
				return deepseek.Invalid(field+".name", "duplicate tool name %q", tool.Name)
			}
			seen[tool.Name] = true
		}
		switch tool.Type {
		case "":
			if tool.Name == "" {
				return deepseek.Invalid(field+".name", "client tools require a name")
			}
			if len(tool.Name) > 128 {
				return deepseek.Invalid(field+".name", "must not exceed 128 characters, got %d", len(tool.Name))
			}
			if !toolNamePattern.MatchString(tool.Name) {
				return deepseek.Invalid(field+".name", "must match %s", toolNamePattern)
			}
			if tool.InputSchema == nil {
				return deepseek.Invalid(field+".input_schema", "client tools require an input schema")
			}
			if typ, ok := tool.InputSchema["type"]; ok {
				name, ok := typ.(string)
				if !ok || name != "object" {
					return deepseek.Invalid(field+".input_schema.type", "must be \"object\"")
				}
			}
		case ToolWebSearch20250305, ToolWebSearch20260209:
			if tool.Name != "web_search" {
				return deepseek.Invalid(field+".name", "web search tools require name \"web_search\"")
			}
			if tool.MaxUses != nil && *tool.MaxUses < 0 {
				return deepseek.Invalid(field+".max_uses", "must not be negative, got %d", *tool.MaxUses)
			}
		case ToolDeepSeekWebSearch:
			if tool.Name != "" && tool.Name != string(ToolDeepSeekWebSearch) {
				return deepseek.Invalid(field+".name", "must be empty or %q", ToolDeepSeekWebSearch)
			}
		case ToolDeepSeekReadURL:
			if tool.Name != "" && tool.Name != string(ToolDeepSeekReadURL) {
				return deepseek.Invalid(field+".name", "must be empty or %q", ToolDeepSeekReadURL)
			}
		default:
			return deepseek.Invalid(field+".type", "unsupported value %q", tool.Type)
		}
	}
	return nil
}

func validateToolChoice(choice ToolChoice) error {
	if choice.Null {
		return nil
	}
	switch choice.Type {
	case ToolChoiceAuto, ToolChoiceAny, ToolChoiceNone:
		return nil
	case ToolChoiceTool:
		if choice.Name == "" {
			return deepseek.Invalid("tool_choice.name", "is required when type is \"tool\"")
		}
		return nil
	case "":
		return deepseek.Invalid("tool_choice.type", "is required")
	default:
		return deepseek.Invalid("tool_choice.type", "must be one of %q, %q, %q or %q, got %q", ToolChoiceAuto, ToolChoiceAny, ToolChoiceTool, ToolChoiceNone, choice.Type)
	}
}

func validateMessages(messages []Message) error {
	for i, message := range messages {
		field := fmt.Sprintf("messages[%d]", i)
		switch message.Role {
		case RoleUser, RoleAssistant, RoleLatestReminder:
		default:
			return deepseek.Invalid(field+".role", "must be one of \"user\", \"assistant\" or \"latest_reminder\", got %q", message.Role)
		}
		if message.Content.Null {
			return deepseek.Invalid(field+".content", "must not be null")
		}
		if message.Content.Blocks != nil && len(message.Content.Blocks) == 0 {
			return deepseek.Invalid(field+".content", "block array must contain at least one block")
		}
		if message.Content.Blocks == nil && message.Content.Text == "" {
			return deepseek.Invalid(field+".content", "must not be empty")
		}
		if message.Role == RoleLatestReminder {
			if message.Content.Blocks != nil {
				return deepseek.Invalid(field+".content", "latest_reminder content must be a string")
			}
			if strings.TrimSpace(message.Content.Text) == "" {
				return deepseek.Invalid(field+".content", "must be a non-empty <date>,<weekday>,<location>,<platform>,<language> string")
			}
			if err := validateReminderContent(field+".content", message.Content.Text); err != nil {
				return err
			}
			if i+1 >= len(messages) || messages[i+1].Role != RoleUser {
				return deepseek.Invalid(field+".role", "latest_reminder must be immediately followed by a user message")
			}
			continue
		}
		for j, block := range message.Content.Blocks {
			if err := validateBlock(fmt.Sprintf("%s.content[%d]", field, j), block); err != nil {
				return err
			}
			if err := validateBlockForRole(fmt.Sprintf("%s.content[%d]", field, j), message.Role, block); err != nil {
				return err
			}
		}
		if message.Role == RoleUser {
			if err := validateToolResultOrder(field, message.Content.Blocks); err != nil {
				return err
			}
		}
	}
	return validateToolPairs(messages)
}

func validateReminderContent(field, content string) error {
	fields := strings.Split(content, ",")
	if len(fields) != 5 {
		return deepseek.Invalid(field, "must be <date>,<weekday>,<location>,<platform>,<language>")
	}
	for _, value := range fields {
		if strings.TrimSpace(value) == "" {
			return deepseek.Invalid(field, "must be <date>,<weekday>,<location>,<platform>,<language>")
		}
	}
	return nil
}

func validateBlock(field string, block Block) error {
	switch block.Type {
	case BlockText:
		if block.Text == "" {
			return deepseek.Invalid(field+".text", "must not be empty")
		}
		return nil
	case BlockImage:
		if block.Source == nil {
			return deepseek.Invalid(field+".source", "image blocks require a source")
		}
		if block.Source.Type == SourceText {
			return deepseek.Invalid(field+".source.type", "image sources must be \"base64\", \"url\" or \"file\"")
		}
		return validateSource(field+".source", *block.Source)
	case BlockDocument:
		if block.Source == nil {
			return deepseek.Invalid(field+".source", "document blocks require a source")
		}
		if block.Source.Type != SourceFile && block.Source.Type != SourceText {
			return deepseek.Invalid(field+".source.type", "document sources must be \"file\" or \"text\", got %q", block.Source.Type)
		}
		return validateSource(field+".source", *block.Source)
	case BlockToolResult:
		if block.ToolUseID == "" {
			return deepseek.Invalid(field+".tool_use_id", "is required")
		}
		if block.Content != nil && block.Content.Blocks != nil {
			for i, inner := range block.Content.Blocks {
				innerField := fmt.Sprintf("%s.content[%d]", field, i)
				switch inner.Type {
				case BlockText:
					if inner.Text == "" {
						return deepseek.Invalid(innerField+".text", "must not be empty")
					}
				case BlockImage:
					if inner.Source == nil {
						return deepseek.Invalid(innerField+".source", "image blocks require a source")
					}
					if inner.Source.Type == SourceText {
						return deepseek.Invalid(innerField+".source.type", "image sources must be \"base64\", \"url\" or \"file\"")
					}
					if err := validateSource(innerField+".source", *inner.Source); err != nil {
						return err
					}
				default:
					return deepseek.Invalid(innerField+".type", "tool_result content supports only text and image blocks, got %q", inner.Type)
				}
			}
		}
		return nil
	case BlockThinking:
		return nil
	case BlockToolUse:
		if block.ID == "" {
			return deepseek.Invalid(field+".id", "is required")
		}
		if block.Name == "" {
			return deepseek.Invalid(field+".name", "is required")
		}
		return nil
	case BlockServerToolUse:
		if block.ID == "" {
			return deepseek.Invalid(field+".id", "is required")
		}
		if block.Name == "" {
			return deepseek.Invalid(field+".name", "is required")
		}
		return nil
	case BlockWebSearchToolResult, BlockWebFetchToolResult:
		return nil
	case "":
		return deepseek.Invalid(field+".type", "is required")
	default:
		// Unknown block types are forwarded unchanged; the server decides.
		return nil
	}
}

func validateBlockForRole(field string, role Role, block Block) error {
	switch block.Type {
	case BlockText:
		return nil
	case BlockImage, BlockDocument, BlockToolResult:
		if role != RoleUser {
			return deepseek.Invalid(field+".type", "%s blocks are only valid in user messages", block.Type)
		}
	case BlockThinking, BlockToolUse, BlockServerToolUse, BlockWebSearchToolResult, BlockWebFetchToolResult:
		if role != RoleAssistant {
			return deepseek.Invalid(field+".type", "%s blocks are only valid in assistant messages", block.Type)
		}
	}
	return nil
}

func validateSource(field string, source Source) error {
	switch source.Type {
	case SourceBase64:
		if source.MediaType == "" {
			return deepseek.Invalid(field+".media_type", "is required for base64 sources")
		}
		if source.Data == "" {
			return deepseek.Invalid(field+".data", "is required for base64 sources")
		}
	case SourceURL:
		if source.URL == "" {
			return deepseek.Invalid(field+".url", "is required for url sources")
		}
	case SourceFile:
		if source.FileID == "" {
			return deepseek.Invalid(field+".file_id", "is required for file sources")
		}
	case SourceText:
		// Data is required by the schema but may legitimately be empty.
	default:
		return deepseek.Invalid(field+".type", "must be one of \"base64\", \"url\", \"file\" or \"text\", got %q", source.Type)
	}
	return nil
}

func validateToolResultOrder(field string, blocks []Block) error {
	seenOther := false
	seenIDs := make(map[string]bool)
	for i, block := range blocks {
		if block.Type != BlockToolResult {
			seenOther = true
			continue
		}
		if seenOther {
			return deepseek.Invalid(fmt.Sprintf("%s.content[%d].type", field, i), "tool_result blocks must come before text, image and document blocks")
		}
		if seenIDs[block.ToolUseID] {
			return deepseek.Invalid(fmt.Sprintf("%s.content[%d].tool_use_id", field, i), "duplicate tool_result for tool_use_id %q", block.ToolUseID)
		}
		seenIDs[block.ToolUseID] = true
	}
	return nil
}

func validateToolPairs(messages []Message) error {
	for i, message := range messages {
		if message.Role != RoleUser {
			continue
		}
		results := toolResultIDs(message.Content)
		if len(results) == 0 {
			continue
		}
		if i == 0 || messages[i-1].Role != RoleAssistant {
			return deepseek.Invalid(fmt.Sprintf("messages[%d].content", i), "tool_result blocks require a preceding assistant message with tool_use blocks")
		}
		uses := toolUseIDs(messages[i-1].Content)
		for id := range results {
			if !uses[id] {
				return deepseek.Invalid(fmt.Sprintf("messages[%d].content", i), "tool_result id %q does not match a tool_use in the previous assistant message", id)
			}
		}
	}
	for i, message := range messages {
		if message.Role != RoleAssistant {
			continue
		}
		uses := toolUseIDs(message.Content)
		if len(uses) == 0 {
			continue
		}
		if i+1 >= len(messages) {
			return deepseek.Invalid(fmt.Sprintf("messages[%d].content", i), "assistant tool_use blocks must be followed by a user message with tool_result blocks")
		}
		if messages[i+1].Role != RoleUser {
			return deepseek.Invalid(fmt.Sprintf("messages[%d].content", i), "assistant tool_use blocks must be followed by a user message with tool_result blocks")
		}
		results := toolResultIDs(messages[i+1].Content)
		for id := range uses {
			if !results[id] {
				return deepseek.Invalid(fmt.Sprintf("messages[%d].content", i), "missing tool_result for tool_use_id %q in the next user message", id)
			}
		}
	}
	return nil
}

func toolUseIDs(content Content) map[string]bool {
	var ids map[string]bool
	for _, block := range content.Blocks {
		if block.Type != BlockToolUse {
			continue
		}
		if ids == nil {
			ids = make(map[string]bool)
		}
		ids[block.ID] = true
	}
	return ids
}

func toolResultIDs(content Content) map[string]bool {
	var ids map[string]bool
	for _, block := range content.Blocks {
		if block.Type != BlockToolResult {
			continue
		}
		if ids == nil {
			ids = make(map[string]bool)
		}
		ids[block.ToolUseID] = true
	}
	return ids
}
