// Command chat_stream streams a Chat Completions answer, printing thinking
// tokens dimmed and the final answer as it arrives.
//
//	go run ./examples/chat_stream "explain Go iterators"
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
	prompt := "Why is the sky blue? Answer in three sentences."
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}

	client := chat.New(ex.NewClient())
	request := &chat.Request{
		Model:           "deepseek-flash",
		Messages:        []chat.Message{chat.UserMessage(prompt)},
		Thinking:        &chat.Thinking{Type: chat.ThinkingEnabled},
		ReasoningEffort: chat.EffortHigh,
	}

	for chunk, err := range client.CreateStream(context.Background(), request) {
		if err != nil {
			fmt.Fprintln(os.Stderr, "stream:", err)
			os.Exit(1)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.ReasoningContent != nil {
			fmt.Print("\x1b[2m", *delta.ReasoningContent, "\x1b[0m")
		}
		if delta.Content != nil {
			fmt.Print(*delta.Content)
		}
	}
	fmt.Println()
}
