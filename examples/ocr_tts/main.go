// Command ocr_tts recognizes text in a document and speaks the first page to a
// WAV file.
//
//	go run ./examples/ocr_tts page.pdf speech.wav
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/tools"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: ocr_tts <document> <output.wav>")
		os.Exit(2)
	}
	inputPath, outputPath := os.Args[1], os.Args[2]

	payload, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}

	ctx := context.Background()
	client := tools.New(ex.NewClient())
	recognized, err := client.OCR(ctx, &tools.OCRRequest{
		File:     payload,
		Filename: filepath.Base(inputPath),
		Purpose:  tools.OCRTextRecognition,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "ocr:", err)
		os.Exit(1)
	}
	if len(recognized.Pages) == 0 {
		fmt.Fprintln(os.Stderr, "ocr: no pages recognized")
		os.Exit(1)
	}

	text := recognized.Pages[0].Text
	fmt.Printf("recognized %d page(s), %d tokens\n", recognized.PageCount, recognized.Usage.TotalTokens)
	fmt.Println(text)

	audio, err := client.Speak(ctx, &tools.TTSRequest{Text: text, VoiceID: tools.TTSVoiceStella})
	if err != nil {
		fmt.Fprintln(os.Stderr, "tts:", err)
		os.Exit(1)
	}
	defer audio.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create:", err)
		os.Exit(1)
	}
	defer out.Close()
	if _, err := io.Copy(out, audio.Reader); err != nil {
		fmt.Fprintln(os.Stderr, "copy:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%s)\n", outputPath, audio.ContentType)
}
