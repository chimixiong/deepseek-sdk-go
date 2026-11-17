// Command search queries the web search tool.
//
//	go run ./examples/search "DeepSeek V4 release"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/tools"
)

func main() {
	query := "DeepSeek API thinking mode"
	if len(os.Args) > 1 {
		query = strings.Join(os.Args[1:], " ")
	}

	response, err := tools.New(ex.NewClient()).Search(context.Background(), &tools.SearchRequest{
		Query: query,
		Count: new(5),
	})
	ex.Must(err)

	for i, result := range response.Results {
		fmt.Printf("%d. %s\n   %s\n   score=%.2f %s\n", i+1, result.Title, result.URL, result.RerankScore, result.Snippet)
	}
}
