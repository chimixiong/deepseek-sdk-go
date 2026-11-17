// Command chat_tools runs a full tool-call loop with thinking enabled. The
// assistant message, including reasoning_content, is replayed on the next turn,
// which DeepSeek requires whenever a request carries tools.
//
//	go run ./examples/chat_tools
package main

import (
	"context"
	"fmt"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/chat"
)

func main() {
	ctx := context.Background()
	client := chat.New(ex.NewClient())

	weather := chat.FunctionTool("get_weather", "Get the current weather for a city.", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{"type": "string", "description": "City name"},
		},
		"required": []string{"city"},
	})
	messages := []chat.Message{
		chat.SystemMessage("Use get_weather for every city. Never answer from memory."),
		chat.UserMessage("What is the weather in Hangzhou and Beijing? Call get_weather for both."),
	}

	for turn := 1; turn <= 5; turn++ {
		response, err := client.Create(ctx, &chat.Request{
			Model:           "deepseek-flash",
			Messages:        messages,
			Tools:           []chat.Tool{weather},
			Thinking:        &chat.Thinking{Type: chat.ThinkingEnabled},
			ReasoningEffort: chat.EffortLow,
			MaxTokens:       new(512),
		})
		ex.Must(err)

		assistant := response.AssistantMessage()
		fmt.Printf("turn %d reasoning: %s\n", turn, response.ReasoningContent())
		if len(assistant.ToolCalls) == 0 {
			fmt.Println(assistant.Content.Text)
			return
		}
		messages = append(messages, assistant)
		for _, call := range assistant.ToolCalls {
			fmt.Printf("  tool call %s %s\n", call.Function.Name, call.Function.Arguments)
			messages = append(messages, chat.ToolMessage(call.ID, chat.TextContent(`{"temperature":"18C","condition":"cloudy"}`)))
		}
	}
	fmt.Println("stopped after 5 turns")
}
