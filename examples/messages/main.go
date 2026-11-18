// Command messages sends one Anthropic-compatible Messages API request with
// thinking enabled, then prints the thinking signature and the answer.
//
//	go run ./examples/messages "what is 2+2?"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/messages"
)

func main() {
	prompt := "What is 2+2? Answer briefly."
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}

	response, err := messages.New(ex.NewClient()).Create(context.Background(), &messages.Request{
		Model: "deepseek-flash",
		Messages: []messages.Message{{
			Role:    messages.RoleUser,
			Content: messages.Content{Text: prompt},
		}},
		MaxTokens:    new(256),
		Thinking:     &messages.Thinking{Type: messages.ThinkingEnabled},
		OutputConfig: &messages.OutputConfig{Effort: messages.EffortLow},
	})
	ex.Must(err)

	for _, block := range response.Content {
		switch block.Type {
		case messages.BlockThinking:
			signature := ""
			if block.Signature != nil {
				signature = *block.Signature
			}
			fmt.Printf("thinking: %s\nsignature: %s\n\n", block.Thinking, signature)
		case messages.BlockText:
			fmt.Println(block.Text)
		}
	}
	stopReason := ""
	if response.StopReason != nil {
		stopReason = string(*response.StopReason)
	}
	fmt.Printf("stop_reason: %s\n", stopReason)
	fmt.Printf("tokens: input=%d output=%d\n", response.Usage.InputTokens, response.Usage.OutputTokens)
}
