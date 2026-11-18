package history

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chimixiong/deepseek-sdk-go/responses"
)

// FromResponse converts a completed Responses API response into a neutral
// transcript. Reasoning items and server-side tool items are dropped with
// warnings because the neutral model has no representation for them.
func FromResponse(response *responses.Response) (Transcript, []Warning) {
	if response == nil {
		return Transcript{}, []Warning{warning(-1, "response", "nil response")}
	}
	transcript, warnings := FromResponsesItems(response.Output)
	if response.Instructions != nil {
		if transcript.System != "" && *response.Instructions != "" {
			transcript.System = transcript.System + "\n\n" + *response.Instructions
			warnings = append(warnings, warning(-1, "system", "instructions joined with the system prompt from output items"))
		} else if *response.Instructions != "" {
			transcript.System = *response.Instructions
		}
	}
	return transcript, warnings
}

// FromResponsesItems converts output items, for example the previous
// response's output, into a neutral transcript.
func FromResponsesItems(items []responses.OutputItem) (Transcript, []Warning) {
	var (
		transcript Transcript
		warnings   []Warning
		system     []string
	)
	for _, item := range items {
		index := len(transcript.Turns)
		switch item.Type {
		case responses.ItemMessage:
			parts, partWarnings := responsesPartsToIR(item.Content, index)
			warnings = append(warnings, partWarnings...)
			switch item.Role {
			case responses.RoleSystem, responses.RoleDeveloper:
				if len(parts) > 0 {
					system = append(system, Texts(parts))
				}
				warnings = append(warnings, warning(-1, "role", "system message moved to the transcript system prompt"))
			case responses.RoleAssistant, "":
				if len(parts) > 0 {
					transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleAssistant, Parts: parts})
				}
			case responses.RoleUser:
				if len(parts) > 0 {
					transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleUser, Parts: parts})
				}
			default:
				warnings = append(warnings, warning(index, "role", fmt.Sprintf("unsupported role %q", item.Role)))
			}
		case responses.ItemFunctionCall:
			if item.CallID == "" {
				warnings = append(warnings, warning(index, "call_id", "function call without call_id dropped"))
				continue
			}
			transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleAssistant, Parts: []Part{
				Call(item.CallID, item.Name, json.RawMessage(item.Arguments)),
			}})
		case responses.ItemReasoning:
			warnings = append(warnings, warning(index, "reasoning", "thinking cannot cross protocols and was dropped"))
		case responses.ItemCustomToolCall, responses.ItemCustomToolCallOutput:
			warnings = append(warnings, warning(index, "custom_tool_call", "custom tool items are not representable"))
		case responses.ItemWebSearchCall, responses.ItemToolSearchCall:
			warnings = append(warnings, warning(index, "server_tool_call", "server-side web tool items are not representable"))
		default:
			warnings = append(warnings, warning(index, "item", fmt.Sprintf("unsupported item type %q", item.Type)))
		}
	}
	if len(system) > 0 {
		transcript.System = strings.Join(system, "\n\n")
	}
	return transcript, warnings
}

// FromResponsesInput converts request input items into a neutral transcript.
// Use it when switching a conversation that was accumulated as Responses input
// items rather than as a Response output.
func FromResponsesInput(items []responses.InputItem) (Transcript, []Warning) {
	var (
		transcript Transcript
		warnings   []Warning
		system     []string
	)
	pending := map[string]bool{}
	for _, item := range items {
		index := len(transcript.Turns)
		switch item.Type {
		case responses.ItemMessage, "":
			var parts []Part
			partWarnings := []Warning(nil)
			if item.Content != nil {
				parts, partWarnings = responsesContentToIR(*item.Content, index)
			}
			warnings = append(warnings, partWarnings...)
			switch item.Role {
			case responses.RoleSystem, responses.RoleDeveloper:
				if len(parts) > 0 {
					system = append(system, Texts(parts))
				}
				warnings = append(warnings, warning(-1, "role", "system message moved to the transcript system prompt"))
			case responses.RoleAssistant:
				if len(parts) > 0 {
					transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleAssistant, Parts: parts})
				}
			case responses.RoleUser, "":
				if len(parts) > 0 {
					transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleUser, Parts: parts})
				}
			default:
				warnings = append(warnings, warning(index, "role", fmt.Sprintf("unsupported role %q", item.Role)))
			}
		case responses.ItemFunctionCall:
			if item.CallID == "" {
				warnings = append(warnings, warning(index, "call_id", "function call without call_id dropped"))
				continue
			}
			pending[item.CallID] = true
			transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleAssistant, Parts: []Part{
				Call(item.CallID, item.Name, json.RawMessage(item.Arguments)),
			}})
		case responses.ItemFunctionCallOutput:
			if !pending[item.CallID] {
				warnings = append(warnings, warning(index, "call_id", fmt.Sprintf("no matching function call %q, output dropped", item.CallID)))
				continue
			}
			delete(pending, item.CallID)
			var parts []Part
			if item.Output != nil {
				converted, partWarnings := responsesContentToIR(*item.Output, index)
				parts, warnings = converted, append(warnings, partWarnings...)
			}
			if len(parts) == 0 {
				parts = []Part{Text("")}
			}
			transcript.Turns = appendTurn(transcript.Turns, Turn{Role: RoleUser, Parts: []Part{
				Result(item.CallID, parts...),
			}})
		case responses.ItemReasoning:
			warnings = append(warnings, warning(index, "reasoning", "thinking cannot cross protocols and was dropped"))
		default:
			warnings = append(warnings, warning(index, "item", fmt.Sprintf("unsupported input item type %q", item.Type)))
		}
	}
	if len(system) > 0 {
		transcript.System = strings.Join(system, "\n\n")
	}
	return transcript, warnings
}

// ToResponses converts a transcript into the instructions field and input
// items. A non-empty transcript system prompt becomes instructions.
func ToResponses(transcript Transcript, options Options) (string, []responses.InputItem, []Warning) {
	var (
		items    []responses.InputItem
		warnings []Warning
	)
	pending := map[string]bool{}
	for _, merged := range mergedTurns(transcript.Turns) {
		index, turn := merged.index, merged.turn
		switch turn.Role {
		case RoleAssistant:
			pending = map[string]bool{}
			text := Texts(turn.Parts)
			if text != "" {
				items = append(items, responses.InputItem{
					Type:    responses.ItemMessage,
					Role:    responses.RoleAssistant,
					Content: &responses.Content{Parts: []responses.ContentPart{responses.OutputTextPart(text)}},
				})
			}
			for _, part := range turn.Parts {
				if part.ToolCall == nil {
					if part.Image != nil || part.File != nil || part.ToolResult != nil {
						warnings = append(warnings, warning(index, "assistant.content", "non-text part dropped from an assistant turn"))
					}
					continue
				}
				if part.ToolCall.ID == "" {
					warnings = append(warnings, warning(index, "call_id", "tool call without id dropped"))
					continue
				}
				if !json.Valid(part.ToolCall.Arguments) {
					warnings = append(warnings, warning(index, "tool_call.arguments", "arguments are not valid JSON"))
				}
				items = append(items, responses.InputItem{
					Type:      responses.ItemFunctionCall,
					CallID:    part.ToolCall.ID,
					Name:      part.ToolCall.Name,
					Arguments: string(part.ToolCall.Arguments),
				})
				pending[part.ToolCall.ID] = true
			}
			if text == "" && !hasToolCall(turn.Parts) {
				warnings = append(warnings, warning(index, "assistant", "empty turn dropped"))
			}
		case RoleUser:
			for _, part := range turn.Parts {
				if part.ToolResult == nil {
					continue
				}
				if part.ToolResult.CallID == "" {
					warnings = append(warnings, warning(index, "tool_result.call_id", "result without call id dropped"))
					continue
				}
				if !pending[part.ToolResult.CallID] {
					if options.DropToolResults {
						warnings = append(warnings, warning(index, "tool_result", "unpaired tool result dropped by option"))
						continue
					}
					warnings = append(warnings, warning(index, "tool_result.call_id", unpairedResultReason(part.ToolResult.CallID)))
				} else {
					delete(pending, part.ToolResult.CallID)
				}
				outputParts, partWarnings := irPartsToResponses(part.ToolResult.Parts, index, options)
				warnings = append(warnings, partWarnings...)
				output := responsesContent(outputParts)
				items = append(items, responses.InputItem{
					Type:   responses.ItemFunctionCallOutput,
					CallID: part.ToolResult.CallID,
					Output: &output,
				})
			}
			messageParts, strayWarnings := userContentParts(turn.Parts, index)
			warnings = append(warnings, strayWarnings...)
			content, partWarnings := irPartsToResponses(messageParts, index, options)
			warnings = append(warnings, partWarnings...)
			if len(content) == 0 {
				continue
			}
			value := responsesContent(content)
			items = append(items, responses.InputItem{
				Type:    responses.ItemMessage,
				Role:    responses.RoleUser,
				Content: &value,
			})
		default:
			warnings = append(warnings, warning(index, "role", fmt.Sprintf("unsupported role %q", turn.Role)))
		}
	}
	return transcript.System, items, warnings
}

func userContentParts(parts []Part, index int) ([]Part, []Warning) {
	var (
		out      []Part
		warnings []Warning
	)
	for _, part := range parts {
		switch {
		case part.ToolResult != nil:
			// Emitted as a function_call_output item.
		case part.ToolCall != nil:
			warnings = append(warnings, warning(index, "tool_call", "tool call outside an assistant turn dropped"))
		default:
			out = append(out, part)
		}
	}
	return out, warnings
}

func hasToolCall(parts []Part) bool {
	for _, part := range parts {
		if part.ToolCall != nil {
			return true
		}
	}
	return false
}

func responsesContent(parts []responses.ContentPart) responses.Content {
	if len(parts) == 1 && (parts[0].Type == responses.PartInputText || parts[0].Type == responses.PartOutputText) {
		return responses.TextContent(parts[0].Text)
	}
	return responses.PartsContent(parts...)
}

func irPartsToResponses(parts []Part, index int, options Options) ([]responses.ContentPart, []Warning) {
	var (
		out      []responses.ContentPart
		warnings []Warning
	)
	for _, part := range parts {
		switch {
		case part.Text != nil:
			out = append(out, responses.InputTextPart(*part.Text))
		case part.Image != nil:
			if options.DropImages {
				warnings = append(warnings, warning(index, "image", "dropped by option"))
				continue
			}
			if !imageHasContent(*part.Image) {
				warnings = append(warnings, warning(index, "image", "image has no URL, data or file id; dropped"))
				continue
			}
			out = append(out, responsesImagePart(*part.Image))
		case part.File != nil:
			if part.File.FileID == "" {
				warnings = append(warnings, warning(index, "file", "Responses input_file requires an uploaded file id; inline document dropped"))
				continue
			}
			out = append(out, responses.InputFilePart(part.File.FileID))
		case part.ToolCall != nil, part.ToolResult != nil:
			warnings = append(warnings, warning(index, "tool_part", "tool part outside a tool item dropped"))
		}
	}
	return out, warnings
}

func responsesImagePart(image Image) responses.ContentPart {
	switch {
	case image.FileID != "":
		return responses.InputImageFilePart(image.FileID, responses.ImageDetail(image.Detail))
	case len(image.Data) > 0:
		mediaType := image.MediaType
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		url := "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(image.Data)
		return responses.InputImagePart(url, responses.ImageDetail(image.Detail))
	default:
		return responses.InputImagePart(image.URL, responses.ImageDetail(image.Detail))
	}
}

func responsesContentToIR(content responses.Content, index int) ([]Part, []Warning) {
	if content.Null {
		return nil, nil
	}
	if content.Parts == nil {
		if content.Text == "" {
			return nil, nil
		}
		return []Part{Text(content.Text)}, nil
	}
	return responsesPartsToIR(content.Parts, index)
}

func responsesPartsToIR(parts []responses.ContentPart, index int) ([]Part, []Warning) {
	var (
		out      []Part
		warnings []Warning
	)
	for _, part := range parts {
		switch part.Type {
		case responses.PartInputText, responses.PartOutputText:
			if part.Text != "" {
				out = append(out, Text(part.Text))
			}
		case responses.PartReasoningText:
			warnings = append(warnings, warning(index, "reasoning_text", "thinking cannot cross protocols and was dropped"))
		case responses.PartInputImage:
			image := Image{FileID: part.FileID, Detail: string(part.Detail)}
			if part.ImageURL != "" {
				if mediaType, data, ok := parseDataURL(part.ImageURL); ok {
					image.Data, image.MediaType = data, mediaType
				} else {
					image.URL = part.ImageURL
				}
			}
			out = append(out, ImagePart(image))
		case responses.PartInputFile:
			if part.FileID == "" {
				warnings = append(warnings, warning(index, "input_file", "input_file without file_id dropped"))
				continue
			}
			out = append(out, FilePart(File{FileID: part.FileID}))
		default:
			if part.Raw != nil {
				warnings = append(warnings, warning(index, "content_part", "unknown part type dropped"))
			}
		}
	}
	return out, warnings
}
