package messages

// MessageType is the type of a Messages API response.
const MessageType = "message"

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
// by message_start.
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
}

// StopDetails explains a refusal stop.
type StopDetails struct {
	Type        string  `json:"type"`
	Category    *string `json:"category,omitempty"`
	Explanation *string `json:"explanation,omitempty"`
}

// RefusalStopDetailsType is the only stop detail type.
const RefusalStopDetailsType = "refusal"
