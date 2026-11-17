// Command messages_stream streams a Messages API answer and prints it as it
// arrives.
//
//	go run ./examples/messages_stream
package main

import (
	"context"
	"fmt"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/messages"
)

func main() {
	response, err := messages.Aggregate(messages.New(ex.NewClient()).CreateStream(context.Background(), &messages.Request{
		Model: "deepseek-flash",
		Messages: []messages.Message{{
			Role:    messages.RoleUser,
			Content: messages.Content{Text: "Count from 1 to 5."},
		}},
		MaxTokens: new(64),
		Thinking:  &messages.Thinking{Type: messages.ThinkingDisabled},
	}))
	ex.Must(err)

	for _, block := range response.Content {
		if block.Type == messages.BlockText {
			fmt.Println(block.Text)
		}
	}
}
