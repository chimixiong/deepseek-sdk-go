package history

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chimixiong/deepseek-sdk-go/messages"
)

// FromMessage converts an assistant Messages API response into a transcript.
// Thinking blocks are dropped with a warning.
func FromMessage(response *messages.Response) (Transcript, []Warning) {
	if response == nil {
		return Transcript{}, []Warning{warning(-1, "response", "nil response")}
	}
	parts, warnings := messagesBlocksToIR(response.Content, 0)
	if len(parts) == 0 {
		return Transcript{}, warnings
	}
	return Transcript{Turns: []Turn{{Role: RoleAssistant, Parts: parts}}}, warnings
}

// FromMessages converts Messages API history and an optional system prompt into
// a transcript. Thinking and server-tool blocks are dropped with warnings.
func FromMessages(msgs []messages.Message, system *messages.Content) (Transcript, []Warning) {
	var (
		transcript Transcript
		warnings   []Warning
	)
	if system != nil {
		transcript.System, warnings = messagesSystem(*system)
	}
	pending := map[string]bool{}
	for _, message := range msgs {
		index := len(transcript.Turns)
		switch message.Role {
		case messages.RoleUser:
			parts, partWarnings := messagesContentToIR(message.Content, index, &pending)
			warnings = append(warnings, partWarnings...)
			if len(parts) == 0 {
				warnings = append(warnings, warning(index, "user", "empty message dropped"))
				continue
			}
			transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleUser, Parts: parts})
		case messages.RoleAssistant:
			parts, partWarnings := messagesBlocksToIR(message.Content.Blocks, index)
			warnings = append(warnings, partWarnings...)
			if message.Content.Blocks == nil && message.Content.Text != "" {
				parts = append(parts, Text(message.Content.Text))
			}
			for _, block := range message.Content.Blocks {
				if block.Type != messages.BlockToolUse {
					continue
				}
				pending[block.ID] = true
			}
			if len(parts) == 0 {
				warnings = append(warnings, warning(index, "assistant", "empty message dropped"))
				continue
			}
			transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleAssistant, Parts: parts})
		case messages.RoleLatestReminder:
			warnings = append(warnings, warning(index, "role", "latest_reminder is not representable"))
		default:
			warnings = append(warnings, warning(index, "role", fmt.Sprintf("unsupported role %q", message.Role)))
		}
	}
	return transcript, warnings
}

// ToMessages converts a transcript into an optional system prompt and Messages
// API messages. Tool result blocks are placed before text and images, as the
// API requires.
func ToMessages(transcript Transcript, options Options) (*messages.Content, []messages.Message, []Warning) {
	var (
		system   *messages.Content
		msgs     []messages.Message
		warnings []Warning
	)
	pending := map[string]bool{}
	if transcript.System != "" {
		system = &messages.Content{Text: transcript.System}
	}
	for index, turn := range transcript.Turns {
		switch turn.Role {
		case RoleAssistant:
			blocks, turnWarnings := irPartsToMessages(turn.Parts, index, options)
			warnings = append(warnings, turnWarnings...)
			var filtered []messages.Block
			for _, block := range blocks {
				if block.Type == messages.BlockText || block.Type == messages.BlockToolUse {
					filtered = append(filtered, block)
				}
			}
			pending = map[string]bool{}
			for _, block := range filtered {
				if block.Type == messages.BlockToolUse && block.ID != "" {
					pending[block.ID] = true
				}
			}
			if len(filtered) != len(blocks) {
				warnings = append(warnings, warning(index, "assistant.content", "only text and tool calls are allowed in an assistant turn"))
			}
			if len(filtered) == 0 {
				warnings = append(warnings, warning(index, "assistant", "empty turn dropped"))
				continue
			}
			msgs = append(msgs, messages.Message{
				Role:    messages.RoleAssistant,
				Content: messages.Content{Blocks: filtered},
			})
		case RoleUser:
			blocks, turnWarnings := messagesUserBlocks(turn.Parts, index, options, pending)
			warnings = append(warnings, turnWarnings...)
			if len(blocks) == 0 {
				warnings = append(warnings, warning(index, "user", "empty turn dropped"))
				continue
			}
			content := messages.Content{Blocks: blocks}
			if len(blocks) == 1 && blocks[0].Type == messages.BlockText {
				content = messages.Content{Text: blocks[0].Text}
			}
			msgs = append(msgs, messages.Message{Role: messages.RoleUser, Content: content})
		default:
			warnings = append(warnings, warning(index, "role", fmt.Sprintf("unsupported role %q", turn.Role)))
		}
	}
	return system, msgs, warnings
}

func messagesUserBlocks(parts []Part, index int, options Options, pending map[string]bool) ([]messages.Block, []Warning) {
	var (
		results  []messages.Block
		content  []messages.Block
		warnings []Warning
	)
	for _, part := range parts {
		switch {
		case part.ToolResult != nil:
			callID := part.ToolResult.CallID
			if callID == "" {
				warnings = append(warnings, warning(index, "tool_result.tool_use_id", "result without call id dropped"))
				continue
			}
			if !pending[callID] {
				if options.DropToolResults {
					warnings = append(warnings, warning(index, "tool_result", "unpaired tool result dropped by option"))
					continue
				}
				warnings = append(warnings, warning(index, "tool_result.tool_use_id", unpairedResultReason(callID)))
			} else {
				delete(pending, callID)
			}
			inner, innerWarnings := irPartsToMessages(part.ToolResult.Parts, index, options)
			warnings = append(warnings, innerWarnings...)
			inner = dropUserToolUse(inner, index, &warnings)
			value := messages.Content{Blocks: inner}
			if len(inner) == 0 {
				value = messages.Content{Text: ""}
			} else if len(inner) == 1 && inner[0].Type == messages.BlockText {
				value = messages.Content{Text: inner[0].Text}
			}
			results = append(results, messages.Block{
				Type:      messages.BlockToolResult,
				ToolUseID: part.ToolResult.CallID,
				Content:   &value,
			})
			continue
		case part.ToolCall != nil:
			warnings = append(warnings, warning(index, "tool_call", "tool call outside an assistant turn dropped"))
			continue
		}
		converted, partWarnings := irPartsToMessages([]Part{part}, index, options)
		warnings = append(warnings, partWarnings...)
		content = append(content, converted...)
	}
	return append(results, content...), warnings
}

// dropUserToolUse removes tool_use blocks, which are invalid inside user
// messages, including inside tool_result content.
func dropUserToolUse(blocks []messages.Block, index int, warnings *[]Warning) []messages.Block {
	var kept []messages.Block
	for _, block := range blocks {
		if block.Type == messages.BlockToolUse {
			*warnings = append(*warnings, warning(index, "tool_use", "tool call inside a user message dropped"))
			continue
		}
		kept = append(kept, block)
	}
	return kept
}

func irPartsToMessages(parts []Part, index int, options Options) ([]messages.Block, []Warning) {
	var (
		blocks   []messages.Block
		warnings []Warning
	)
	for _, part := range parts {
		switch {
		case part.Text != nil:
			blocks = append(blocks, messages.Block{Type: messages.BlockText, Text: *part.Text})
		case part.Image != nil:
			if options.DropImages {
				warnings = append(warnings, warning(index, "image", "dropped by option"))
				continue
			}
			if !imageHasContent(*part.Image) {
				warnings = append(warnings, warning(index, "image", "image has no URL, data or file id; dropped"))
				continue
			}
			blocks = append(blocks, messages.Block{Type: messages.BlockImage, Source: messagesImageSource(*part.Image)})
		case part.File != nil:
			block, ok := messagesDocumentBlock(*part.File)
			if !ok {
				warnings = append(warnings, warning(index, "document", "inline binary document without a file id dropped"))
				continue
			}
			blocks = append(blocks, block)
		case part.ToolCall != nil:
			if part.ToolCall.ID == "" {
				warnings = append(warnings, warning(index, "tool_use.id", "tool call without id dropped"))
				continue
			}
			input, ok := toolInput(part.ToolCall.Arguments)
			if !ok {
				warnings = append(warnings, warning(index, "tool_use.input", "arguments are not a JSON object; sent {}"))
			}
			blocks = append(blocks, messages.Block{
				Type:  messages.BlockToolUse,
				ID:    part.ToolCall.ID,
				Name:  part.ToolCall.Name,
				Input: input,
			})
		case part.ToolResult != nil:
			warnings = append(warnings, warning(index, "tool_result", "nested tool result dropped"))
		}
	}
	return blocks, warnings
}

func toolInput(arguments json.RawMessage) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(arguments)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return json.RawMessage("{}"), false
	}
	return append(json.RawMessage(nil), trimmed...), true
}

func messagesImageSource(image Image) *messages.Source {
	switch {
	case image.FileID != "":
		return &messages.Source{Type: messages.SourceFile, FileID: image.FileID}
	case len(image.Data) > 0:
		mediaType := image.MediaType
		if mediaType == "" {
			mediaType = "image/jpeg"
		}
		return &messages.Source{
			Type:      messages.SourceBase64,
			MediaType: mediaType,
			Data:      base64.StdEncoding.EncodeToString(image.Data),
		}
	default:
		return &messages.Source{Type: messages.SourceURL, URL: image.URL}
	}
}

func messagesDocumentBlock(file File) (messages.Block, bool) {
	if file.FileID != "" {
		var title *string
		if file.Filename != "" {
			title = &file.Filename
		}
		return messages.Block{
			Type:   messages.BlockDocument,
			Source: &messages.Source{Type: messages.SourceFile, FileID: file.FileID},
			Title:  title,
		}, true
	}
	if len(file.Data) > 0 && (file.MediaType == "" || strings.HasPrefix(file.MediaType, "text/")) {
		return messages.Block{
			Type: messages.BlockDocument,
			Source: &messages.Source{
				Type:      messages.SourceText,
				MediaType: "text/plain",
				Data:      string(file.Data),
			},
		}, true
	}
	return messages.Block{}, false
}

func messagesSystem(content messages.Content) (string, []Warning) {
	if content.Null {
		return "", nil
	}
	if content.Blocks == nil {
		return content.Text, nil
	}
	var (
		texts    []string
		warnings []Warning
	)
	for _, block := range content.Blocks {
		if block.Type == messages.BlockText {
			texts = append(texts, block.Text)
			continue
		}
		warnings = append(warnings, warning(-1, "system.content", "non-text block dropped"))
	}
	return strings.Join(texts, "\n\n"), warnings
}

func messagesContentToIR(content messages.Content, index int, pending *map[string]bool) ([]Part, []Warning) {
	if content.Null {
		return nil, nil
	}
	if content.Blocks == nil {
		if content.Text == "" {
			return nil, nil
		}
		return []Part{Text(content.Text)}, nil
	}
	var (
		parts    []Part
		warnings []Warning
	)
	for _, block := range content.Blocks {
		if block.Type == messages.BlockToolResult {
			if block.ToolUseID == "" || !(*pending)[block.ToolUseID] {
				warnings = append(warnings, warning(index, "tool_result.tool_use_id", "no matching tool_use, result dropped"))
				continue
			}
			delete(*pending, block.ToolUseID)
			inner := []Part(nil)
			if block.Content != nil {
				converted, innerWarnings := messagesContentToIR(*block.Content, index, pending)
				inner, warnings = converted, append(warnings, innerWarnings...)
			}
			if len(inner) == 0 {
				inner = []Part{Text("")}
			}
			parts = append(parts, Result(block.ToolUseID, inner...))
			continue
		}
		converted, blockWarnings := messagesBlocksToIR([]messages.Block{block}, index)
		warnings = append(warnings, blockWarnings...)
		parts = append(parts, converted...)
	}
	return parts, warnings
}

func messagesBlocksToIR(blocks []messages.Block, index int) ([]Part, []Warning) {
	var (
		parts    []Part
		warnings []Warning
	)
	for _, block := range blocks {
		switch block.Type {
		case messages.BlockText:
			if len(block.Citations) > 0 {
				warnings = append(warnings, warning(index, "text.citations", "citations dropped"))
			}
			if block.Text != "" {
				parts = append(parts, Text(block.Text))
			}
		case messages.BlockImage:
			if block.Source == nil {
				warnings = append(warnings, warning(index, "image.source", "missing source dropped"))
				continue
			}
			image, reason := messagesSourceToImage(*block.Source)
			if reason != "" {
				warnings = append(warnings, warning(index, "image.source", reason))
				continue
			}
			parts = append(parts, ImagePart(image))
		case messages.BlockDocument:
			if block.Source == nil {
				warnings = append(warnings, warning(index, "document.source", "missing source dropped"))
				continue
			}
			file, ok := messagesSourceToFile(*block.Source, block.Title)
			if !ok {
				warnings = append(warnings, warning(index, "document", "unsupported document source dropped"))
				continue
			}
			parts = append(parts, FilePart(file))
		case messages.BlockToolUse:
			parts = append(parts, Call(block.ID, block.Name, block.Input))
		case messages.BlockThinking:
			warnings = append(warnings, warning(index, "thinking", "thinking cannot cross protocols and was dropped"))
		case messages.BlockServerToolUse:
			warnings = append(warnings, warning(index, "server_tool_use", "server-side tool block is not representable"))
		case messages.BlockWebSearchToolResult, messages.BlockWebFetchToolResult:
			warnings = append(warnings, warning(index, "server_tool_result", "server-side tool result is not representable"))
		case messages.BlockToolResult:
			warnings = append(warnings, warning(index, "tool_result", "tool result must appear in a user message"))
		default:
			if block.Raw != nil {
				warnings = append(warnings, warning(index, "block", "unknown block type dropped"))
			}
		}
	}
	return parts, warnings
}

// messagesSourceToImage converts an image source, returning a non-empty reason
// when the image cannot be represented without fabricating an empty URL.
func messagesSourceToImage(source messages.Source) (Image, string) {
	switch source.Type {
	case messages.SourceBase64:
		if source.Data == "" {
			return Image{}, "base64 source has no data; image dropped"
		}
		data, err := base64.StdEncoding.DecodeString(source.Data)
		if err != nil {
			return Image{}, "base64 source data is invalid; image dropped"
		}
		return Image{Data: data, MediaType: source.MediaType}, ""
	case messages.SourceURL:
		if source.URL == "" {
			return Image{}, "url source is empty; image dropped"
		}
		return Image{URL: source.URL}, ""
	case messages.SourceFile:
		if source.FileID == "" {
			return Image{}, "file source has no file id; image dropped"
		}
		return Image{FileID: source.FileID}, ""
	default:
		return Image{}, fmt.Sprintf("unsupported source type %q dropped", source.Type)
	}
}

func messagesSourceToFile(source messages.Source, title *string) (File, bool) {
	filename := ""
	if title != nil {
		filename = *title
	}
	switch source.Type {
	case messages.SourceFile:
		return File{FileID: source.FileID, Filename: filename}, true
	case messages.SourceText:
		return File{Data: []byte(source.Data), MediaType: "text/plain", Filename: filename}, true
	default:
		return File{}, false
	}
}
