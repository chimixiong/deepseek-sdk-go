// Command responses_stream streams a Responses API answer, printing reasoning
// tokens dimmed and the answer as it arrives.
//
//	go run ./examples/responses_stream
package main

import (
	"context"
	"fmt"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/responses"
)

func main() {
	client := responses.New(ex.NewClient())
	events := client.CreateStream(context.Background(), &responses.Request{
		Model:           "deepseek-flash",
		Input:           responses.StringInput("Count from 1 to 5."),
		Reasoning:       &responses.Reasoning{Effort: responses.EffortLow},
		MaxOutputTokens: new(128),
	})
	for event, err := range events {
		ex.Must(err)
		switch event.Type {
		case responses.EventReasoningTextDelta:
			fmt.Print("\x1b[2m", event.Delta, "\x1b[0m")
		case responses.EventOutputTextDelta:
			fmt.Print(event.Delta)
		case responses.EventFailed:
			if event.Response != nil && event.Response.Error != nil {
				fmt.Printf("\nstream failed: %+v\n", event.Response.Error)
				continue
			}
			fmt.Printf("\nstream failed\n")
		}
	}
	fmt.Println()
}
