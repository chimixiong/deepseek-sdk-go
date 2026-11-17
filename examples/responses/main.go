// Command responses sends one Responses API request and prints the output text.
//
//	go run ./examples/responses "say hi"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/responses"
)

func main() {
	prompt := "Explain Go iterators in one sentence."
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}

	response, err := responses.New(ex.NewClient()).Create(context.Background(), &responses.Request{
		Model:           "deepseek-flash",
		Instructions:    new("Answer in plain text."),
		Input:           responses.StringInput(prompt),
		Reasoning:       &responses.Reasoning{Effort: responses.EffortNone},
		MaxOutputTokens: new(128),
	})
	ex.Must(err)

	fmt.Println(response.OutputText())
	if usage := response.Usage; usage != nil {
		fmt.Printf("\ntokens: input=%d output=%d total=%d\n", usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
	}
}
