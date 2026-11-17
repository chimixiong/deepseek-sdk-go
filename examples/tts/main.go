// Command tts synthesizes speech and writes it to a WAV file. The output path
// defaults to speech.wav, which examples/asr reads by default.
//
//	go run ./examples/tts "hello from DeepSeek" [speech.wav]
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/tools"
)

func main() {
	text := "Hello from the DeepSeek Go SDK."
	output := "speech.wav"
	if len(os.Args) > 1 {
		text = os.Args[1]
	}
	if len(os.Args) > 2 {
		output = os.Args[2]
	}

	audio, err := tools.New(ex.NewClient()).Speak(context.Background(), &tools.TTSRequest{
		Text:    text,
		VoiceID: tools.TTSVoiceStella,
		Format:  tools.TTSFormatWAV,
	})
	ex.Must(err)
	defer audio.Close()

	file, err := os.Create(output)
	ex.Must(err)
	defer file.Close()
	written, err := io.Copy(file, audio.Reader)
	ex.Must(err)
	fmt.Printf("wrote %s (%s, %d bytes)\n", output, audio.ContentType, written)
}
