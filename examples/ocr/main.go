// Command ocr recognizes text page by page. With no argument it uses a
// generated one-page PDF containing "Hello DeepSeek OCR".
//
//	go run ./examples/ocr [path/to/document.pdf]
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/tools"
)

func main() {
	payload := ex.SamplePDF()
	filename := "sample.pdf"
	if len(os.Args) > 1 {
		path := os.Args[1]
		data, err := os.ReadFile(path)
		ex.Must(err)
		payload = data
		filename = filepath.Base(path)
	}

	response, err := tools.New(ex.NewClient()).OCR(context.Background(), &tools.OCRRequest{
		File:     payload,
		Filename: filename,
		Purpose:  tools.OCRTextRecognition,
	})
	ex.Must(err)

	fmt.Printf("%s: %d page(s), %d tokens\n", response.ID, response.PageCount, response.Usage.TotalTokens)
	for _, page := range response.Pages {
		fmt.Printf("--- page %d (%s) ---\n%s\n", page.Index, page.FinishReason, page.Text)
	}
}
