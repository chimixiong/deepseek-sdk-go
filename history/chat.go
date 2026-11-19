package history

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/chimixiong/deepseek-sdk-go/chat"
)

// FromChat converts Chat Completions history into the neutral transcript.
// Reasoning content cannot be represented and is dropped with a warning.
func FromChat(messages []chat.Message) (Transcript, []Warning) {
	var (
		transcript Transcript
		warnings   []Warning
		system     []string
	)
	pendingCalls := map[string]bool{}

	for _, message := range messages {
		switch message.Role {
		case chat.RoleSystem:
			system = append(system, chatText(message.Content, &warnings, -1)...)
			if message.Content.Parts != nil {
				warnings = append(warnings, warning(-1, "system.content", "blocks flattened to text"))
			}
		case chat.RoleUser:
			parts, partWarnings := chatPartsToIR(message.Content, len(transcript.Turns))
			warnings = append(warnings, partWarnings...)
			if len(parts) == 0 {
				warnings = append(warnings, warning(len(transcript.Turns), "user.content", "empty message dropped"))
				continue
			}
			transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleUser, Parts: parts})
		case chat.RoleAssistant:
			if message.ReasoningContent != nil && *message.ReasoningContent != "" {
				warnings = append(warnings, warning(len(transcript.Turns), "reasoning_content", "thinking cannot cross protocols and was dropped"))
			}
			var parts []Part
			if message.Content.Null {
				// Tool-only assistant turn.
			} else if text := strings.TrimSpace(message.Content.Text); text != "" || message.Content.Parts == nil {
				if message.Content.Text != "" {
					parts = append(parts, Text(message.Content.Text))
				}
			}
			if message.Content.Parts != nil {
				converted, partWarnings := chatPartsToIR(message.Content, len(transcript.Turns))
				warnings = append(warnings, partWarnings...)
				parts = append(parts, converted...)
			}
			if len(message.ServerToolCalls) > 0 {
				warnings = append(warnings, warning(len(transcript.Turns), "server_tool_calls", "server-side tool calls are not representable"))
			}
			if message.Refusal != "" {
				warnings = append(warnings, warning(len(transcript.Turns), "refusal", "refusal marker dropped"))
			}
			pendingCalls = map[string]bool{}
			for _, call := range message.ToolCalls {
				parts = append(parts, Call(call.ID, call.Function.Name, json.RawMessage(call.Function.Arguments)))
				pendingCalls[call.ID] = true
			}
			if len(parts) == 0 {
				warnings = append(warnings, warning(len(transcript.Turns), "assistant", "empty message dropped"))
				continue
			}
			transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleAssistant, Parts: parts})
		case chat.RoleTool:
			if message.ToolCallID == "" {
				warnings = append(warnings, warning(len(transcript.Turns), "tool.tool_call_id", "missing call id, result dropped"))
				continue
			}
			if !pendingCalls[message.ToolCallID] {
				warnings = append(warnings, warning(len(transcript.Turns), "tool.tool_call_id", fmt.Sprintf("no matching tool call %q, result dropped", message.ToolCallID)))
				continue
			}
			delete(pendingCalls, message.ToolCallID)
			result, partWarnings := chatToolResult(message, len(transcript.Turns))
			warnings = append(warnings, partWarnings...)
			transcript.Turns = appendTurn(transcript.Turns, Turn{
				Role:  RoleUser,
				Parts: []Part{result},
			})
		default:
			warnings = append(warnings, warning(len(transcript.Turns), "role", fmt.Sprintf("message role %q is not representable", message.Role)))
		}
	}

	if len(system) > 0 {
		transcript.System = strings.Join(system, "\n\n")
	}
	if len(system) > 1 {
		warnings = append(warnings, warning(-1, "system", "multiple system messages joined"))
	}
	return transcript, warnings
}

// ToChat converts a neutral transcript into Chat Completions messages.
func ToChat(transcript Transcript, options Options) ([]chat.Message, []Warning) {
	var (
		messages []chat.Message
		warnings []Warning
	)
	pending := map[string]bool{}
	if transcript.System != "" {
		messages = append(messages, chat.SystemMessage(transcript.System))
	}
	for _, merged := range mergedTurns(transcript.Turns) {
		index, turn := merged.index, merged.turn
		switch turn.Role {
		case RoleAssistant:
			pending = map[string]bool{}
			message := chat.Message{Role: chat.RoleAssistant, Content: chat.TextContent(Texts(turn.Parts))}
			for _, part := range turn.Parts {
				if part.ToolCall == nil {
					if part.Image != nil || part.File != nil || part.ToolResult != nil {
						warnings = append(warnings, warning(index, "assistant.content", "non-text part dropped from an assistant turn"))
					}
					continue
				}
				arguments, ok := toolInput(part.ToolCall.Arguments)
				if !ok {
					warnings = append(warnings, warning(index, "tool_call.arguments", "arguments are not a JSON object; sent {}"))
				}
				message.ToolCalls = append(message.ToolCalls, chat.ToolCall{
					ID:       part.ToolCall.ID,
					Type:     "function",
					Function: chat.FunctionCall{Name: part.ToolCall.Name, Arguments: string(arguments)},
				})
				if part.ToolCall.ID != "" {
					pending[part.ToolCall.ID] = true
				}
			}
			if message.Content.Text == "" && len(message.ToolCalls) == 0 {
				warnings = append(warnings, warning(index, "assistant", "empty turn dropped"))
				continue
			}
			messages = append(messages, message)
		case RoleUser:
			toolMessages, userParts, turnWarnings := chatUserTurn(turn.Parts, index, options, pending)
			warnings = append(warnings, turnWarnings...)
			messages = append(messages, toolMessages...)
			if len(userParts) == 0 {
				if len(toolMessages) == 0 {
					warnings = append(warnings, warning(index, "user", "empty turn dropped"))
				}
				continue
			}
			messages = append(messages, chat.Message{
				Role:    chat.RoleUser,
				Content: chatContent(userParts),
			})
		default:
			warnings = append(warnings, warning(index, "role", fmt.Sprintf("unsupported role %q", turn.Role)))
		}
	}
	return messages, warnings
}

func chatUserTurn(parts []Part, index int, options Options, pending map[string]bool) ([]chat.Message, []chat.ContentPart, []Warning) {
	var (
		tools    []chat.Message
		content  []chat.ContentPart
		warnings []Warning
	)
	for _, part := range parts {
		switch {
		case part.ToolResult != nil:
			callID := part.ToolResult.CallID
			if callID == "" {
				warnings = append(warnings, warning(index, "tool_result.call_id", "result without call id dropped"))
				continue
			}
			if !pending[callID] {
				if options.DropToolResults {
					warnings = append(warnings, warning(index, "tool_result", "unpaired tool result dropped by option"))
					continue
				}
				warnings = append(warnings, warning(index, "tool_result.call_id", unpairedResultReason(callID)))
			} else {
				delete(pending, callID)
			}
			tools = append(tools, chat.ToolMessage(callID, chatToolContent(part.ToolResult.Parts, index, options, &warnings)))
		case part.ToolCall != nil:
			warnings = append(warnings, warning(index, "tool_call", "tool call outside an assistant turn dropped"))
		default:
			converted, partWarnings := irPartToChat(part, index, options)
			warnings = append(warnings, partWarnings...)
			content = append(content, converted...)
		}
	}
	return tools, content, warnings
}

func chatToolResult(message chat.Message, index int) (Part, []Warning) {
	var warnings []Warning
	parts, partWarnings := chatPartsToIR(message.Content, index)
	warnings = append(warnings, partWarnings...)
	if len(parts) == 0 {
		parts = []Part{Text("")}
	}
	return Result(message.ToolCallID, parts...), warnings
}

func chatToolContent(parts []Part, index int, options Options, warnings *[]Warning) chat.Content {
	var content []chat.ContentPart
	for _, part := range parts {
		converted, partWarnings := irPartToChat(part, index, options)
		*warnings = append(*warnings, partWarnings...)
		content = append(content, converted...)
	}
	return chatContent(content)
}

func chatContent(parts []chat.ContentPart) chat.Content {
	if len(parts) == 0 {
		// Tool results may legitimately carry no output. An empty string is the
		// documented representation and validates, unlike a nil block array.
		return chat.TextContent("")
	}
	if len(parts) == 1 && parts[0].Type == chat.PartText {
		return chat.TextContent(parts[0].Text)
	}
	return chat.PartsContent(parts...)
}

func chatPartsToIR(content chat.Content, index int) ([]Part, []Warning) {
	var (
		parts    []Part
		warnings []Warning
	)
	if content.Null {
		return nil, nil
	}
	if content.Parts == nil {
		if content.Text == "" {
			return nil, nil
		}
		return []Part{Text(content.Text)}, nil
	}
	for _, block := range content.Parts {
		switch {
		case block.Type == chat.PartText:
			if block.Text != "" {
				parts = append(parts, Text(block.Text))
			}
		case block.Type == chat.PartImage && block.ImageURL != nil:
			if block.ImageURL.URL == "" {
				warnings = append(warnings, warning(index, "image_url.url", "image URL is empty, image dropped"))
				continue
			}
			parts = append(parts, ImagePart(Image{URL: block.ImageURL.URL, Detail: block.ImageURL.Detail}))
		case block.Type == chat.PartFile && block.File != nil:
			file, reason := chatFileToIR(*block.File)
			if reason != "" {
				warnings = append(warnings, warning(index, "file", reason))
				continue
			}
			parts = append(parts, FilePart(file))
		default:
			warnings = append(warnings, warning(index, "content_part", "unknown block type dropped"))
		}
	}
	return parts, warnings
}

func chatFileToIR(file chat.FileRef) (File, string) {
	if file.FileID != "" {
		return File{FileID: file.FileID, Filename: file.Filename}, ""
	}
	if file.FileData == "" {
		return File{}, "file reference has neither file_id nor file_data; dropped"
	}
	mediaType, data, ok := parseDataURL(file.FileData)
	if !ok {
		return File{}, "file_data is not a valid base64 or percent-encoded data URL; dropped"
	}
	return File{Data: data, MediaType: mediaType, Filename: file.Filename}, ""
}

func irPartToChat(part Part, index int, options Options) ([]chat.ContentPart, []Warning) {
	switch {
	case part.Text != nil:
		return []chat.ContentPart{chat.TextPart(*part.Text)}, nil
	case part.Image != nil:
		if options.DropImages {
			return nil, []Warning{warning(index, "image", "dropped by option")}
		}
		if !imageHasContent(*part.Image) {
			return nil, []Warning{warning(index, "image", "image has no URL, data or file id; dropped")}
		}
		converted, reason := chatImagePart(*part.Image)
		if reason != "" {
			return nil, []Warning{warning(index, "image", reason)}
		}
		return []chat.ContentPart{converted}, nil
	case part.File != nil:
		if part.File.FileID == "" && len(part.File.Data) == 0 {
			return nil, []Warning{warning(index, "file", "file has no file id or data; dropped")}
		}
		return []chat.ContentPart{chat.FilePart(chatFileRef(*part.File))}, nil
	default:
		return nil, []Warning{warning(index, "part", "part is not representable in Chat Completions")}
	}
}

func chatImagePart(image Image) (chat.ContentPart, string) {
	switch {
	case image.FileID != "":
		return chat.FilePart(chat.FileRef{FileID: image.FileID}), ""
	case len(image.Data) > 0:
		mediaType, ok := imageMediaType(image)
		if !ok {
			return chat.ContentPart{}, "inline image media type could not be determined; dropped"
		}
		url := "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(image.Data)
		return chat.ImagePart(url, image.Detail), ""
	default:
		return chat.ImagePart(image.URL, image.Detail), ""
	}
}

func chatFileRef(file File) chat.FileRef {
	if file.FileID != "" {
		// filename is only valid with file_data; the service rejects the pair.
		return chat.FileRef{FileID: file.FileID}
	}
	mediaType := file.MediaType
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	return chat.FileRef{
		FileData: "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(file.Data),
		Filename: file.Filename,
	}
}

func chatText(content chat.Content, warnings *[]Warning, index int) []string {
	if content.Null {
		return nil
	}
	if content.Parts == nil {
		return []string{content.Text}
	}
	var texts []string
	for _, block := range content.Parts {
		if block.Type == chat.PartText && block.Text != "" {
			texts = append(texts, block.Text)
			continue
		}
		*warnings = append(*warnings, warning(index, "system.content", "non-text block dropped"))
	}
	return texts
}

// parseDataURL decodes an RFC 2397 data URL into its media type and payload.
// The payload is base64 when a ";base64" parameter is present, otherwise it is
// percent-encoded text.
func parseDataURL(value string) (string, []byte, bool) {
	if !strings.HasPrefix(value, "data:") {
		return "", nil, false
	}
	header, payload, found := strings.Cut(strings.TrimPrefix(value, "data:"), ",")
	if !found {
		return "", nil, false
	}
	segments := strings.Split(header, ";")
	mediaType := strings.TrimSpace(segments[0])
	base64Encoded := false
	for _, param := range segments[1:] {
		if strings.EqualFold(strings.TrimSpace(param), "base64") {
			base64Encoded = true
		}
	}
	if base64Encoded {
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload))
		if err != nil {
			return "", nil, false
		}
		return mediaType, data, true
	}
	data, err := url.PathUnescape(payload)
	if err != nil {
		return "", nil, false
	}
	return mediaType, []byte(data), true
}

func appendTurn(turns []Turn, turn Turn) []Turn {
	if len(turns) > 0 && turns[len(turns)-1].Role == turn.Role {
		turns[len(turns)-1].Parts = append(turns[len(turns)-1].Parts, turn.Parts...)
		return turns
	}
	return append(turns, turn)
}
