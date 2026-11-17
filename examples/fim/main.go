// Command fim demonstrates fill-in-the-middle completion on the beta endpoint.
//
//	go run ./examples/fim
package main

import (
	"context"
	"fmt"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/fim"
)

func main() {
	suffix := "    return result\n"
	response, err := fim.New(ex.NewClient()).Create(context.Background(), &fim.Request{
		Model:     "deepseek-flash",
		Prompt:    "def add(a, b):\n    result = a + b\n",
		Suffix:    &suffix,
		MaxTokens: new(32),
	})
	ex.Must(err)
	if len(response.Choices) == 0 {
		ex.Must(fmt.Errorf("no completion choices"))
	}
	fmt.Println(response.Choices[0].Text)
}
