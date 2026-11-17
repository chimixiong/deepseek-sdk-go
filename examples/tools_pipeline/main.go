// Command tools_pipeline searches the web and asks a model to summarize the
// results, exercising the tools and chat packages together.
//
//	go run ./examples/tools_pipeline "DeepSeek V4 release"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/chat"
	"github.com/chimixiong/deepseek-sdk-go/tools"
)

func main() {
	query := "DeepSeek API thinking mode"
	if len(os.Args) > 1 {
		query = strings.Join(os.Args[1:], " ")
	}
	ctx := context.Background()
	core := ex.NewClient()

	search, err := tools.New(core).Search(ctx, &tools.SearchRequest{
		Query: query,
		Count: new(5),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "search:", err)
		os.Exit(1)
	}

	var contextText strings.Builder
	for i, result := range search.Results {
		fmt.Fprintf(&contextText, "%d. %s\n%s\n%s\n\n", i+1, result.Title, result.URL, result.Snippet)
		fmt.Printf("%d. %s\n   %s\n", i+1, result.Title, result.URL)
	}

	answer, err := chat.New(core).Create(ctx, &chat.Request{
		Model: "deepseek-flash",
		Messages: []chat.Message{
			chat.SystemMessage("Use only the supplied search results. Reply in Chinese."),
			chat.UserMessage("总结这些结果：\n\n" + contextText.String()),
		},
		Thinking: &chat.Thinking{Type: chat.ThinkingDisabled},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "chat:", err)
		os.Exit(1)
	}
	fmt.Println("\n" + answer.AssistantMessage().Content.Text)
}
