// Command chat_beta demonstrates assistant prefix continuation, which requires
// the beta endpoint.
//
//	go run ./examples/chat_beta
package main

import (
	"context"
	"fmt"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/chat"
)

func main() {
	prefix := true
	response, err := chat.NewBeta(ex.NewClient()).Create(context.Background(), &chat.Request{
		Model: "deepseek-flash",
		Messages: []chat.Message{
			chat.UserMessage("Continue the sequence with three more numbers."),
			{Role: chat.RoleAssistant, Content: chat.TextContent("1, 2, 3,"), Prefix: &prefix},
		},
		Thinking:  &chat.Thinking{Type: chat.ThinkingDisabled},
		MaxTokens: new(16),
	})
	ex.Must(err)
	fmt.Println(response.AssistantMessage().Content.Text)
}
