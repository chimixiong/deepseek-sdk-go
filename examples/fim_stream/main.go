// Command fim_stream streams a fill-in-the-middle completion from the beta
// endpoint.
//
//	go run ./examples/fim_stream
package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/fim"
)

func main() {
	suffix := "    return result\n"
	stream, err := fim.New(ex.NewClient()).CreateStream(context.Background(), &fim.Request{
		Model:     "deepseek-flash",
		Prompt:    "def add(a, b):\n    result = a + b\n",
		Suffix:    &suffix,
		MaxTokens: new(32),
	})
	ex.Must(err)
	defer stream.Close()

	for {
		chunk, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		ex.Must(err)
		for _, choice := range chunk.Choices {
			fmt.Print(choice.Text)
		}
	}
	fmt.Println()
}
