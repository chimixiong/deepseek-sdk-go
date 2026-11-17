// Command fetch downloads a page and prints its Markdown content.
//
//	go run ./examples/fetch [url]
package main

import (
	"context"
	"fmt"
	"os"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/tools"
)

func main() {
	url := "https://api-docs.deepseek.com/zh-cn/api/create-chat-completion"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	page, err := tools.New(ex.NewClient()).Fetch(context.Background(), &tools.FetchRequest{URL: url})
	ex.Must(err)

	fmt.Printf("title: %s\n\n", page.Title)
	content := page.Content
	if len(content) > 1000 {
		content = content[:1000] + "\n... (truncated)"
	}
	fmt.Println(content)
}
