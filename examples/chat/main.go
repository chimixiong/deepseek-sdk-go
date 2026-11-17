// Command chat sends one non-streaming Chat Completions request with thinking
// enabled and prints the reasoning, answer and usage.
//
//	go run ./examples/chat "why is the sky blue?"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/chat"
)

func main() {
	prompt := "Why is the sky blue? Answer in two sentences."
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}

	client := chat.New(ex.NewClient())
	response, err := client.Create(context.Background(), &chat.Request{
		Model:           "deepseek-flash",
		Messages:        []chat.Message{chat.UserMessage(prompt)},
		Thinking:        &chat.Thinking{Type: chat.ThinkingEnabled},
		ReasoningEffort: chat.EffortLow,
		MaxTokens:       new(512),
	})
	ex.Must(err)

	fmt.Printf("reasoning: %s\n\n", response.ReasoningContent())
	fmt.Println(response.AssistantMessage().Content.Text)
	if usage := response.Usage; usage != nil {
		fmt.Printf("\ntokens: prompt=%d completion=%d total=%d cached=%d\n",
			usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, usage.PromptTokensDetails.CachedTokens)
	}
}
