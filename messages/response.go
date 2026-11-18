package messages

import deepseek "github.com/chimixiong/deepseek-sdk-go"

// MessageType is the type of a Messages API response.
const MessageType = "message"

// TypeError is the type of a failed generation body.
const TypeError = "error"

// StopReason says why generation ended.
type StopReason string

// Stop reasons.
const (
	StopReasonEndTurn      StopReason = "end_turn"
	StopReasonMaxTokens    StopReason = "max_tokens"
	StopReasonStopSequence StopReason = "stop_sequence"
	StopReasonToolUse      StopReason = "tool_use"
	StopReasonRefusal      StopReason = "refusal"
)

// Response is a Messages API message, both non-streaming and the value carried
// by message_start. A failed generation keeps the 200 status line and returns a
// body of type "error" with the billed usage; Create returns it as an error
// alongside the decoded response.
type Response struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Role         Role         `json:"role"`
	Model        string       `json:"model"`
	Content      []Block      `json:"content"`
	StopReason   *StopReason  `json:"stop_reason"`
	StopSequence *string      `json:"stop_sequence"`
	StopDetails  *StopDetails `json:"stop_details,omitempty"`
	Title        *string      `json:"title,omitempty"`
	Usage        Usage        `json:"usage"`
	// Error is set on a generation failure returned with a 200 status line.
	Error *deepseek.ResponseError `json:"error,omitempty"`
}

// StopDetails explains a refusal stop.
type StopDetails struct {
	Type        string  `json:"type"`
	Category    *string `json:"category,omitempty"`
	Explanation *string `json:"explanation,omitempty"`
}

// RefusalStopDetailsType is the only stop detail type.
const RefusalStopDetailsType = "refusal"
