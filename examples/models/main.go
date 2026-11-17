// Command models lists the models available to the account.
//
//	go run ./examples/models
package main

import (
	"context"
	"fmt"
	"strings"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/models"
)

func main() {
	list, err := models.New(ex.NewClient()).List(context.Background())
	ex.Must(err)

	for _, model := range list.Data {
		fmt.Printf("%s (%s, owned by %s)\n", model.ID, model.Name, model.OwnedBy)
		if model.ContextWindow != nil {
			fmt.Printf("  context_window=%d\n", *model.ContextWindow)
		}
		if model.MaxOutputTokens != nil {
			fmt.Printf("  max_output_tokens=%d\n", *model.MaxOutputTokens)
		}
		if model.Effort != nil {
			levels := strings.Join(model.Effort.SupportedLevels, ",")
			defaultLevel := ""
			if model.Effort.DefaultLevel != nil {
				defaultLevel = *model.Effort.DefaultLevel
			}
			fmt.Printf("  effort levels=%s default=%s\n", levels, defaultLevel)
		}
		fmt.Printf("  input=%v output=%v\n", model.InputModalities, model.OutputModalities)
	}
}
