package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSpeak(t *testing.T) {
	audioBytes := []byte("RIFF\x24\x00\x00\x00WAVEfmt fake wav payload")
	var (
		gotAccept string
		gotBody   map[string]any
	)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(audioBytes)
	})

	filter := false
	audio, err := c.Speak(context.Background(), &TTSRequest{
		Text:           "Hello from DeepSeek.",
		VoiceID:        TTSVoiceStella,
		Format:         TTSFormatWAV,
		FilterMarkdown: &filter,
	})
	if err != nil {
		t.Fatalf("Speak: %v", err)
	}
	defer audio.Close()

	if gotAccept != "audio/*" {
		t.Fatalf("Accept = %q, want audio/*", gotAccept)
	}
	if gotBody["text"] != "Hello from DeepSeek." {
		t.Fatalf("text = %v", gotBody["text"])
	}
	if gotBody["voice_id"] != "stella" || gotBody["format"] != "wav" {
		t.Fatalf("voice_id/format = %v/%v", gotBody["voice_id"], gotBody["format"])
	}
	if gotBody["filter_markdown"] != false {
		t.Fatalf("filter_markdown = %v, want false", gotBody["filter_markdown"])
	}
	if audio.ContentType != "audio/wav" {
		t.Fatalf("Content-Type = %q, want audio/wav", audio.ContentType)
	}
	data, err := io.ReadAll(audio.Reader)
	if err != nil {
		t.Fatalf("read audio: %v", err)
	}
	if !bytes.Equal(data, audioBytes) {
		t.Fatalf("audio = %q, want %q", data, audioBytes)
	}
}

func TestSpeakPCMBytes(t *testing.T) {
	audioBytes := []byte{0x00, 0x01, 0xfe, 0xff, 0x10, 0x20}
	var gotBody map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "audio/pcm")
		w.Write(audioBytes)
	})

	data, contentType, err := c.SpeakBytes(context.Background(), &TTSRequest{
		Text:    "raw pcm",
		VoiceID: TTSVoiceTide,
		Format:  TTSFormatPCM,
	})
	if err != nil {
		t.Fatalf("SpeakBytes: %v", err)
	}
	if contentType != "audio/pcm" {
		t.Fatalf("Content-Type = %q, want audio/pcm", contentType)
	}
	if !bytes.Equal(data, audioBytes) {
		t.Fatalf("audio = %v, want %v", data, audioBytes)
	}
	if gotBody["voice_id"] != "tide" || gotBody["format"] != "pcm" {
		t.Fatalf("voice_id/format = %v/%v", gotBody["voice_id"], gotBody["format"])
	}
	if _, ok := gotBody["filter_markdown"]; ok {
		t.Fatalf("filter_markdown should be omitted, body = %v", gotBody)
	}
}

func TestTTSDefaultsOmitted(t *testing.T) {
	var gotBody map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("x"))
	})
	audio, err := c.Speak(context.Background(), &TTSRequest{Text: "hi"})
	if err != nil {
		t.Fatalf("Speak: %v", err)
	}
	audio.Close()
	for _, field := range []string{"voice_id", "format", "filter_markdown"} {
		if _, ok := gotBody[field]; ok {
			t.Errorf("%s should be omitted so the server default applies, body = %v", field, gotBody)
		}
	}
}

func TestTTSValidation(t *testing.T) {
	c := newTestClient(t, noRequest(t))
	tests := []struct {
		name  string
		req   *TTSRequest
		field string
	}{
		{"nil request", nil, "request"},
		{"empty text", &TTSRequest{}, "text"},
		{"whitespace text", &TTSRequest{Text: "  \n\t "}, "text"},
		{"unknown voice", &TTSRequest{Text: "hi", VoiceID: "bogus"}, "voice_id"},
		{"unknown format", &TTSRequest{Text: "hi", Format: "mp3"}, "format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Speak(context.Background(), tt.req)
			requireValidationError(t, err, tt.field)
		})
	}
}

func TestTTSAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"message":"text too long","type":"invalid_request_error"}}`)
	})
	_, err := c.Speak(context.Background(), &TTSRequest{Text: "hi"})
	apiErr := requireAPIError(t, err, http.StatusBadRequest)
	if apiErr.Message != "text too long" {
		t.Fatalf("message = %q", apiErr.Message)
	}
}

func TestSpeakRejectsNonAudioContentType(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<html>proxy error</html>")
	})
	audio, err := c.Speak(context.Background(), &TTSRequest{Text: "hi"})
	if err == nil {
		audio.Close()
		t.Fatal("Speak accepted a non-audio response")
	}
	if !strings.Contains(err.Error(), "text/html") {
		t.Fatalf("error = %v, want it to name text/html", err)
	}

	data, contentType, err := c.SpeakBytes(context.Background(), &TTSRequest{Text: "hi"})
	if err == nil {
		t.Fatalf("SpeakBytes returned data=%q contentType=%q, want error", data, contentType)
	}
}

func TestSpeakAcceptsContentTypeParameters(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav; rate=24000")
		w.Write([]byte("wav"))
	})
	audio, err := c.Speak(context.Background(), &TTSRequest{Text: "hi"})
	if err != nil {
		t.Fatalf("Speak: %v", err)
	}
	defer audio.Close()
	if audio.ContentType != "audio/wav; rate=24000" {
		t.Fatalf("Content-Type = %q", audio.ContentType)
	}
}

func TestSpeakJSONErrorBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-request-id", "req-tts")
		io.WriteString(w, `{"error":{"message":"text did not pass content review","type":"invalid_request_error","code":"content_filter"}}`)
	})
	_, err := c.Speak(context.Background(), &TTSRequest{Text: "hi"})
	apiErr := requireAPIError(t, err, http.StatusOK)
	if !strings.Contains(apiErr.Message, "content review") || apiErr.RequestID != "req-tts" {
		t.Fatalf("api error = %+v", apiErr)
	}
	if apiErr.Code != "content_filter" {
		t.Fatalf("code = %q", apiErr.Code)
	}
}

func TestSpeakBytesDetectsShortAudio(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/pcm")
		w.Header().Set("Content-Length", "100")
		w.Write(make([]byte, 10))
	})
	if _, _, err := c.SpeakBytes(context.Background(), &TTSRequest{Text: "hi", Format: TTSFormatPCM}); err == nil {
		t.Fatal("SpeakBytes accepted a body shorter than Content-Length")
	}
}

func TestSpeakBytesValidatesWAVHeader(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		// RIFF header claims 100 bytes; the body ends immediately after WAVE.
		w.Write([]byte("RIFF\x64\x00\x00\x00WAVE"))
	})
	if _, _, err := c.SpeakBytes(context.Background(), &TTSRequest{Text: "hi"}); err == nil {
		t.Fatal("SpeakBytes accepted a truncated WAV body")
	}
}

func TestTTSBilledCharacterLimit(t *testing.T) {
	c := newTestClient(t, noRequest(t))
	chinese := strings.Repeat("中", maxTTSBilledChars/2+1)
	_, err := c.Speak(context.Background(), &TTSRequest{Text: chinese})
	requireValidationError(t, err, "text")

	ascii := strings.Repeat("a", maxTTSBilledChars+1)
	_, err = c.Speak(context.Background(), &TTSRequest{Text: ascii})
	requireValidationError(t, err, "text")
}

func TestTTSBodyLimit(t *testing.T) {
	c := newTestClient(t, noRequest(t))
	_, err := c.Speak(context.Background(), &TTSRequest{
		Text:  "hi",
		Extra: map[string]json.RawMessage{"padding": json.RawMessage(`"` + strings.Repeat("x", maxTTSBody) + `"`)},
	})
	requireValidationError(t, err, "body")
}
