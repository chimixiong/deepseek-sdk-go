package tools

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode"

	"github.com/chimixiong/deepseek-sdk-go"
)

const (
	maxTTSBilledChars = 10000
	maxTTSBody        = 64 << 10
	maxTTSErrorBody   = 64 << 10
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
	if billed := billedChars(r.Text); billed > maxTTSBilledChars {
		return deepseek.Invalid("text", "must be at most %d billed characters, got %d", maxTTSBilledChars, billed)
	}
	if encoded, err := json.Marshal(r); err != nil {
		return deepseek.Invalid("body", "cannot encode request: %v", err)
	} else if len(encoded) > maxTTSBody {
		return deepseek.Invalid("body", "must be at most %d bytes, got %d", maxTTSBody, len(encoded))
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

// billedChars returns the billed length of text: a Chinese character counts as
// two characters, any other rune as one.
func billedChars(text string) int {
	count := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			count += 2
			continue
		}
		count++
	}
	return count
}

// Audio is a streaming text-to-speech response. The caller must Close Reader,
// which also releases the HTTP connection.
type Audio struct {
	// ContentType is the response media type, for example "audio/wav" or
	// "audio/pcm".
	ContentType string
	// ContentLength is the advertised body length, or -1 when unknown.
	ContentLength int64
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
		data, _ := io.ReadAll(io.LimitReader(resp.Body, maxTTSErrorBody))
		resp.Body.Close()
		return nil, ttsResponseError(resp, data, contentType)
	}
	reader := io.ReadCloser(resp.Body)
	if resp.ContentLength >= 0 {
		reader = &exactReader{ReadCloser: resp.Body, remaining: resp.ContentLength}
	}
	return &Audio{ContentType: contentType, ContentLength: resp.ContentLength, Reader: reader}, nil
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
	if audio.ContentLength >= 0 && int64(len(data)) != audio.ContentLength {
		return nil, "", fmt.Errorf("tools: audio truncated: got %d of %d bytes: %w", len(data), audio.ContentLength, io.ErrUnexpectedEOF)
	}
	if strings.HasPrefix(audio.ContentType, "audio/wav") {
		if err := validateWAV(data); err != nil {
			return nil, "", err
		}
	}
	return data, audio.ContentType, nil
}

// exactReader turns an early EOF into io.ErrUnexpectedEOF when Content-Length
// promised more bytes.
type exactReader struct {
	io.ReadCloser
	remaining int64
}

func (r *exactReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.remaining -= int64(n)
	if err == io.EOF && r.remaining > 0 {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}

// validateWAV rejects a WAV body whose RIFF or data chunk is shorter than its
// header claims.
func validateWAV(data []byte) error {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return fmt.Errorf("tools: WAV body is truncated: %w", io.ErrUnexpectedEOF)
	}
	riffSize := uint64(binary.LittleEndian.Uint32(data[4:8]))
	if riffSize != 0 && riffSize != 0xFFFFFFFF && riffSize+8 > uint64(len(data)) {
		return fmt.Errorf("tools: WAV RIFF chunk claims %d bytes but body has %d: %w", riffSize+8, len(data), io.ErrUnexpectedEOF)
	}
	for offset := 12; offset+8 <= len(data); {
		id := string(data[offset : offset+4])
		size := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if id == "data" && size != 0xFFFFFFFF && size > uint64(len(data)-offset) {
			return fmt.Errorf("tools: WAV data chunk claims %d bytes but body has %d: %w", size, len(data)-offset, io.ErrUnexpectedEOF)
		}
		if size == 0xFFFFFFFF || size > uint64(len(data)-offset) {
			return nil
		}
		offset += int(size)
		if offset%2 == 1 {
			offset++
		}
	}
	return nil
}

func ttsResponseError(resp *http.Response, body []byte, contentType string) error {
	var wire struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
			Param   string `json:"param"`
		} `json:"error"`
	}
	apiErr := &deepseek.APIError{
		StatusCode: resp.StatusCode,
		RequestID: firstNonEmpty(
			resp.Header.Get("x-request-id"),
			resp.Header.Get("x-ds-trace-id"),
			resp.Header.Get("request-id"),
		),
		Body: body,
	}
	if json.Unmarshal(body, &wire) == nil && wire.Error != nil {
		apiErr.Message, apiErr.Type, apiErr.Code, apiErr.Param =
			wire.Error.Message, wire.Error.Type, wire.Error.Code, wire.Error.Param
	} else {
		apiErr.Message = strings.TrimSpace(string(body))
	}
	if apiErr.Message == "" {
		apiErr.Message = "response is not audio"
	}
	apiErr.Message = fmt.Sprintf("unexpected response Content-Type %q, want audio/*: %s", contentType, apiErr.Message)
	return apiErr
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
