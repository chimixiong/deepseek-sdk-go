// Command asr transcribes an audio file. The path defaults to speech.wav,
// which examples/tts generates.
//
//	go run ./examples/asr [speech.wav]
package main

import (
	"context"
	"fmt"
	"os"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/tools"
)

func main() {
	path := "speech.wav"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	payload, err := os.ReadFile(path)
	ex.Must(err)

	response, err := tools.New(ex.NewClient()).ASR(context.Background(), &tools.ASRRequest{
		File:     payload,
		Filename: path,
	})
	ex.Must(err)

	fmt.Printf("%q\n", response.Text)
	fmt.Printf("duration: %d ms, request: %s\n", response.DurationMS, response.RequestID)
}
