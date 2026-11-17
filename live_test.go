//go:build live

package deepseek_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chimixiong/deepseek-sdk-go"
	"github.com/chimixiong/deepseek-sdk-go/billing"
	"github.com/chimixiong/deepseek-sdk-go/chat"
	"github.com/chimixiong/deepseek-sdk-go/files"
	"github.com/chimixiong/deepseek-sdk-go/fim"
	"github.com/chimixiong/deepseek-sdk-go/messages"
	"github.com/chimixiong/deepseek-sdk-go/models"
	"github.com/chimixiong/deepseek-sdk-go/responses"
	"github.com/chimixiong/deepseek-sdk-go/tools"
)

func liveClient(t *testing.T) *deepseek.Client {
	t.Helper()
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY is not set")
	}
	return deepseek.New(deepseek.Config{MaxRetries: 1, RetryBaseDelay: time.Second})
}

func liveContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 3*time.Minute)
}

func TestLiveModelsAndBalance(t *testing.T) {
	core := liveClient(t)
	ctx, cancel := liveContext(t)
	defer cancel()

	list, err := models.New(core).List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Data) == 0 {
		t.Fatal("no models returned")
	}
	for _, model := range list.Data {
		if model.ID == "" || model.Object == "" || model.OwnedBy == "" {
			t.Errorf("incomplete model: %+v", model)
		}
	}
	found := false
	for _, model := range list.Data {
		if model.ContextWindow != nil && *model.ContextWindow > 0 {
			found = true
		}
	}
	if !found {
		t.Error("no model reported a context window")
	}

	balance, err := billing.New(core).GetBalance(ctx)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if len(balance.BalanceInfos) == 0 {
		t.Fatal("no balance info returned")
	}
	t.Logf("available=%v balance=%+v", balance.IsAvailable, balance.BalanceInfos)
}

func TestLiveChat(t *testing.T) {
	core := liveClient(t)
	client := chat.New(core)
	ctx, cancel := liveContext(t)
	defer cancel()

	response, err := client.Create(ctx, &chat.Request{
		Model:     "deepseek-flash",
		Messages:  []chat.Message{chat.UserMessage("Reply with exactly: ok")},
		Thinking:  &chat.Thinking{Type: chat.ThinkingDisabled},
		MaxTokens: new(32),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if strings.TrimSpace(response.AssistantMessage().Content.Text) == "" {
		t.Fatalf("empty answer: %+v", response)
	}
	if response.Usage == nil || response.Usage.TotalTokens == 0 {
		t.Errorf("usage = %+v", response.Usage)
	}
	t.Logf("answer=%q usage=%+v", response.AssistantMessage().Content.Text, response.Usage)

	streamed, err := chat.Accumulate(client.CreateStream(ctx, &chat.Request{
		Model:     "deepseek-flash",
		Messages:  []chat.Message{chat.UserMessage("Count from 1 to 5.")},
		Thinking:  &chat.Thinking{Type: chat.ThinkingDisabled},
		MaxTokens: new(64),
	}))
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if streamed.AssistantMessage().Content.Text == "" {
		t.Fatal("stream produced no text")
	}
	if streamed.Usage == nil {
		t.Error("stream produced no usage")
	}
	t.Logf("streamed=%q", streamed.AssistantMessage().Content.Text)
}

func TestLiveChatToolReasoningRoundTrip(t *testing.T) {
	core := liveClient(t)
	client := chat.New(core)
	ctx, cancel := liveContext(t)
	defer cancel()

	weather := chat.FunctionTool("get_weather", "Get the current weather for a city.", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{"type": "string"},
		},
		"required": []string{"city"},
	})
	request := &chat.Request{
		Model: "deepseek-flash",
		Messages: []chat.Message{
			chat.SystemMessage("Use the get_weather tool. Never answer from memory."),
			chat.UserMessage("What is the weather in Hangzhou? Call get_weather."),
		},
		Tools:           []chat.Tool{weather},
		Thinking:        &chat.Thinking{Type: chat.ThinkingEnabled},
		ReasoningEffort: chat.EffortLow,
		MaxTokens:       new(512),
	}
	first, err := client.Create(ctx, request)
	if err != nil {
		t.Fatalf("first turn: %v", err)
	}
	assistant := first.AssistantMessage()
	if len(assistant.ToolCalls) == 0 {
		t.Skipf("model did not call the tool; finish_reason=%s", first.Choices[0].FinishReason)
	}
	call := assistant.ToolCalls[0]
	t.Logf("reasoning=%q tool=%s args=%s", first.ReasoningContent(), call.Function.Name, call.Function.Arguments)

	request.Messages = append(request.Messages,
		assistant,
		chat.ToolMessage(call.ID, chat.TextContent(`{"temperature":"18C","condition":"cloudy"}`)),
	)
	second, err := client.Create(ctx, request)
	if err != nil {
		t.Fatalf("second turn with reasoning_content replay: %v", err)
	}
	if second.Choices[0].FinishReason == chat.FinishInsufficientSystemRes {
		t.Fatalf("second turn interrupted: %+v", second)
	}
	t.Logf("second answer=%q", second.AssistantMessage().Content.Text)
}

func TestLiveChatBetaPrefix(t *testing.T) {
	core := liveClient(t)
	ctx, cancel := liveContext(t)
	defer cancel()

	prefix := true
	response, err := chat.NewBeta(core).Create(ctx, &chat.Request{
		Model: "deepseek-flash",
		Messages: []chat.Message{
			chat.UserMessage("Continue the sequence with three more numbers."),
			{Role: chat.RoleAssistant, Content: chat.TextContent("1, 2, 3,"), Prefix: &prefix},
		},
		Thinking:  &chat.Thinking{Type: chat.ThinkingDisabled},
		MaxTokens: new(16),
	})
	if err != nil {
		t.Fatalf("beta prefix: %v", err)
	}
	text := response.AssistantMessage().Content.Text
	if text == "" {
		t.Fatal("prefix completion returned no text")
	}
	t.Logf("continuation=%q", text)
}

func TestLiveResponses(t *testing.T) {
	core := liveClient(t)
	client := responses.New(core)
	ctx, cancel := liveContext(t)
	defer cancel()

	response, err := client.Create(ctx, &responses.Request{
		Model:           "deepseek-flash",
		Input:           responses.StringInput("Reply with exactly: ok"),
		Reasoning:       &responses.Reasoning{Effort: responses.EffortNone},
		MaxOutputTokens: new(64),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if strings.TrimSpace(response.OutputText()) == "" {
		t.Fatalf("empty output: %+v", response)
	}
	if response.Usage == nil || response.Usage.TotalTokens == 0 {
		t.Errorf("usage = %+v", response.Usage)
	}

	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"answer": map[string]any{"type": "string"}},
		"required":   []string{"answer"},
	}
	structured, err := client.Create(ctx, &responses.Request{
		Model: "deepseek-flash",
		Input: responses.StringInput("Answer with JSON containing the word ok."),
		Text: &responses.Text{Format: &responses.TextFormat{
			Type:   responses.FormatJSONSchema,
			Name:   "answer",
			Schema: schema,
		}},
		MaxOutputTokens: new(128),
	})
	if err != nil {
		t.Fatalf("json_schema: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(structured.OutputText()), &parsed); err != nil {
		t.Fatalf("structured output is not JSON: %v (%q)", err, structured.OutputText())
	}
	if _, ok := parsed["answer"]; !ok {
		t.Errorf("structured output = %v", parsed)
	}

	streamed, err := responses.Accumulate(client.CreateStream(ctx, &responses.Request{
		Model:           "deepseek-flash",
		Input:           responses.StringInput("Say hi."),
		Reasoning:       &responses.Reasoning{Effort: responses.EffortNone},
		MaxOutputTokens: new(32),
	}))
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if streamed.OutputText() == "" {
		t.Fatal("stream produced no text")
	}
	t.Logf("responses answer=%q structured=%s streamed=%q", response.OutputText(), structured.OutputText(), streamed.OutputText())
}

func TestLiveMessages(t *testing.T) {
	core := liveClient(t)
	client := messages.New(core)
	ctx, cancel := liveContext(t)
	defer cancel()

	response, err := client.Create(ctx, &messages.Request{
		Model: "deepseek-flash",
		Messages: []messages.Message{{
			Role:    messages.RoleUser,
			Content: messages.Content{Text: "Reply with exactly: ok"},
		}},
		MaxTokens: new(32),
		Thinking:  &messages.Thinking{Type: messages.ThinkingDisabled},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if strings.TrimSpace(messagesText(response)) == "" {
		t.Fatalf("empty answer: %+v", response)
	}

	thinking, err := client.Create(ctx, &messages.Request{
		Model: "deepseek-flash",
		Messages: []messages.Message{{
			Role:    messages.RoleUser,
			Content: messages.Content{Text: "What is 2+2? Answer briefly."},
		}},
		MaxTokens:    new(128),
		Thinking:     &messages.Thinking{Type: messages.ThinkingEnabled},
		OutputConfig: &messages.OutputConfig{Effort: messages.EffortLow},
	})
	if err != nil {
		t.Fatalf("thinking: %v", err)
	}
	hasThinking := false
	for _, block := range thinking.Content {
		if block.Type == messages.BlockThinking {
			hasThinking = block.Signature != nil && *block.Signature != ""
		}
	}
	t.Logf("messages answer=%q thinking_block_with_signature=%v", messagesText(thinking), hasThinking)

	streamed, err := messages.Aggregate(client.CreateStream(ctx, &messages.Request{
		Model: "deepseek-flash",
		Messages: []messages.Message{{
			Role:    messages.RoleUser,
			Content: messages.Content{Text: "Count from 1 to 5."},
		}},
		MaxTokens: new(64),
		Thinking:  &messages.Thinking{Type: messages.ThinkingDisabled},
	}))
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if strings.TrimSpace(messagesText(streamed)) == "" {
		t.Fatal("stream produced no text")
	}
	t.Logf("messages streamed=%q", messagesText(streamed))
}

func TestLiveFIM(t *testing.T) {
	core := liveClient(t)
	ctx, cancel := liveContext(t)
	defer cancel()

	suffix := "    return result\n"
	response, err := fim.New(core).Create(ctx, &fim.Request{
		Model:     "deepseek-flash",
		Prompt:    "def add(a, b):\n    result = a + b\n",
		Suffix:    &suffix,
		MaxTokens: new(16),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Text) == "" {
		t.Fatalf("empty completion: %+v", response)
	}
	t.Logf("fim=%q", response.Choices[0].Text)
}

func TestLiveFiles(t *testing.T) {
	core := liveClient(t)
	client := files.New(core)
	ctx, cancel := liveContext(t)
	defer cancel()

	file, err := client.Create(ctx, &files.CreateRequest{
		Purpose:     files.PurposeUserData,
		File:        testPNG(t),
		Filename:    "dot.png",
		ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if file.ID == "" || file.Bytes == 0 {
		t.Fatalf("file = %+v", file)
	}
	deleted := false
	defer func() {
		if !deleted {
			if _, err := client.Delete(ctx, file.ID); err != nil {
				t.Logf("cleanup delete: %v", err)
			}
		}
	}()

	list, err := client.List(ctx, &files.ListOptions{Limit: new(10)})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, item := range list.Data {
		if item.ID == file.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("uploaded file not in list: %+v", list.Data)
	}

	retrieved, err := client.Retrieve(ctx, file.ID)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if retrieved.ID != file.ID || retrieved.Filename != "dot.png" {
		t.Errorf("retrieved = %+v", retrieved)
	}

	deletion, err := client.Delete(ctx, file.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	deleted = true
	if !deletion.Deleted {
		t.Errorf("deletion = %+v", deletion)
	}
	t.Logf("file=%+v list=%d", file, len(list.Data))
}

func TestLiveTools(t *testing.T) {
	core := liveClient(t)
	client := tools.New(core)
	ctx, cancel := liveContext(t)
	defer cancel()

	search, err := client.Search(ctx, &tools.SearchRequest{Query: "DeepSeek API thinking mode", Count: new(3)})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(search.Results) == 0 {
		t.Fatal("search returned no results")
	}
	if search.Results[0].Title == "" || search.Results[0].URL == "" {
		t.Errorf("search result = %+v", search.Results[0])
	}

	fetched, err := client.Fetch(ctx, &tools.FetchRequest{URL: "https://api-docs.deepseek.com/zh-cn/api/create-chat-completion"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if fetched.Title == "" || fetched.Content == "" {
		t.Errorf("fetch = %+v", fetched)
	}

	audio, contentType, err := client.SpeakBytes(ctx, &tools.TTSRequest{
		Text:   "Hello from the DeepSeek Go SDK live test.",
		Format: tools.TTSFormatWAV,
	})
	if err != nil {
		t.Fatalf("TTS: %v", err)
	}
	if !strings.Contains(contentType, "audio") || len(audio) < 44 {
		t.Fatalf("tts content_type=%q bytes=%d", contentType, len(audio))
	}

	transcript, err := client.ASR(ctx, &tools.ASRRequest{File: audio, Filename: "speech.wav"})
	if err != nil {
		t.Fatalf("ASR: %v", err)
	}
	if strings.TrimSpace(transcript.Text) == "" {
		t.Errorf("ASR returned no text: %+v", transcript)
	}

	pdf := testPDF(t)
	var ocr *tools.OCRResponse
	ocrText := ""
	for _, purpose := range []tools.OCRPurpose{tools.OCRTextRecognition, tools.OCRRichLayoutRecognition} {
		result, err := client.OCR(ctx, &tools.OCRRequest{
			File:     pdf,
			Filename: "sample.pdf",
			Purpose:  purpose,
		})
		if err != nil {
			t.Fatalf("OCR(%s): %v", purpose, err)
		}
		if result.PageCount == 0 || len(result.Pages) == 0 {
			t.Fatalf("OCR(%s) = %+v", purpose, result)
		}
		ocr = result
		if text := strings.TrimSpace(result.Pages[0].Text); text != "" {
			ocrText = text
		}
		t.Logf("OCR(%s) pages=%d text=%q", purpose, result.PageCount, result.Pages[0].Text)
	}
	if ocrText == "" {
		t.Error("OCR returned no text for either purpose")
	}
	t.Logf("search=%d fetch=%d tts=%d asr=%q ocr_pages=%d ocr_text=%q",
		len(search.Results), len(fetched.Content), len(audio), transcript.Text, ocr.PageCount, ocrText)
}

func messagesText(response *messages.Response) string {
	if response == nil {
		return ""
	}
	var builder strings.Builder
	for _, block := range response.Content {
		if block.Type == messages.BlockText {
			builder.WriteString(block.Text)
		}
	}
	return builder.String()
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.White)
		}
	}
	for y := 16; y < 48; y++ {
		for x := 16; x < 48; x++ {
			img.Set(x, y, color.Black)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func testPDF(t *testing.T) []byte {
	t.Helper()
	content := "BT /F1 24 Tf 72 700 Td (Hello DeepSeek OCR) Tj ET"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return buf.Bytes()
}
