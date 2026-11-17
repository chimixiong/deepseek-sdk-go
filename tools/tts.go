package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/chimixiong/deepseek-sdk-go"
)

// TTSVoice selects a text-to-speech voice.
type TTSVoice string

const (
	// TTSVoiceStella is the female voice and the server default.
	TTSVoiceStella TTSVoice = "stella"
	// TTSVoiceTide is the male voice.
	TTSVoiceTide TTSVoice = "tide"
)

// TTSFormat selects the audio encoding of the response.
type TTSFormat string

const (
	// TTSFormatWAV is a WAV file served as audio/wav and is the server default.
	TTSFormatWAV TTSFormat = "wav"
	// TTSFormatPCM is raw 24 kHz, 16-bit signed, mono PCM served as audio/pcm.
	TTSFormatPCM TTSFormat = "pcm"
)

// TTSRequest is the body of POST /v1/tools/tts.
type TTSRequest struct {
	// Text is the text to speak. It must not be empty.
	Text string `json:"text"`
	// VoiceID defaults to TTSVoiceStella on the server.
	VoiceID TTSVoice `json:"voice_id,omitempty"`
	// Format defaults to TTSFormatWAV on the server.
	Format TTSFormat `json:"format,omitempty"`
	// FilterMarkdown strips Markdown before speaking. The server defaults to
	// true.
	FilterMarkdown *bool `json:"filter_markdown,omitempty"`
	// Extra adds top-level fields the SDK does not model.
	Extra map[string]json.RawMessage `json:"-"`
}

// Validate reports the documented request constraints.
func (r *TTSRequest) Validate() error {
	if r == nil {
		return deepseek.Invalid("request", "must not be nil")
	}
	if strings.TrimSpace(r.Text) == "" {
		return deepseek.Invalid("text", "must not be empty")
	}
	if r.VoiceID != "" && r.VoiceID != TTSVoiceStella && r.VoiceID != TTSVoiceTide {
		return deepseek.Invalid("voice_id", "must be %q or %q", TTSVoiceStella, TTSVoiceTide)
	}
	if r.Format != "" && r.Format != TTSFormatWAV && r.Format != TTSFormatPCM {
		return deepseek.Invalid("format", "must be %q or %q", TTSFormatWAV, TTSFormatPCM)
	}
	return nil
}

// MarshalJSON adds Extra fields to the encoded request.
func (r *TTSRequest) MarshalJSON() ([]byte, error) {
	type plain TTSRequest
	return deepseek.MarshalWithExtra((*plain)(r), r.Extra)
}

// Audio is a streaming text-to-speech response. The caller must Close Reader,
// which also releases the HTTP connection.
type Audio struct {
	// ContentType is the response media type, for example "audio/wav" or
	// "audio/pcm".
	ContentType string
	// Reader streams the audio bytes. It is never nil on success.
	Reader io.ReadCloser
}

// Close closes the audio stream.
func (a *Audio) Close() error {
	if a == nil || a.Reader == nil {
		return nil
	}
	return a.Reader.Close()
}

// Speak synthesizes speech and returns the response body unread, so audio can
// be played or copied while it is still being produced. A successful response
// must have an audio/* Content-Type; any other media type (for example an HTML
// page or a JSON error from a proxy) is returned as an error and the body is
// closed.
func (c *Client) Speak(ctx context.Context, req *TTSRequest) (*Audio, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	client, err := c.transport()
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(ctx, &deepseek.Request{
		Method: http.MethodPost,
		URL:    client.ToolURL("tts"),
		Header: http.Header{"Accept": {"audio/*"}},
		JSON:   req,
	})
	if err != nil {
		return nil, err
	}
	contentType := resp.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "audio/") {
		resp.Body.Close()
		return nil, fmt.Errorf("tools: unexpected response Content-Type %q, want audio/*", contentType)
	}
	return &Audio{ContentType: contentType, Reader: resp.Body}, nil
}

// SpeakBytes synthesizes speech and reads the complete response into memory.
// It returns the audio bytes and the response Content-Type.
func (c *Client) SpeakBytes(ctx context.Context, req *TTSRequest) ([]byte, string, error) {
	audio, err := c.Speak(ctx, req)
	if err != nil {
		return nil, "", err
	}
	defer audio.Close()
	data, err := io.ReadAll(audio.Reader)
	if err != nil {
		return nil, "", err
	}
	return data, audio.ContentType, nil
}
