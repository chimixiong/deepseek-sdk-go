package chat_test

import (
	"encoding/json"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go/chat"
)

func TestContentUnionMarshal(t *testing.T) {
	cases := []struct {
		name    string
		content chat.Content
		want    string
	}{
		{"text", chat.TextContent("hi"), `"hi"`},
		{"null", chat.NullContent(), `null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.content)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}

	parts := chat.PartsContent(chat.TextPart("a"), chat.ImagePart("https://example.com/x.png", "high"))
	got, err := json.Marshal(parts)
	if err != nil {
		t.Fatalf("marshal parts: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded[0]["type"] != "text" || decoded[1]["image_url"].(map[string]any)["detail"] != "high" {
		t.Fatalf("parts = %s", got)
	}
}

func TestContentUnknownPartRoundTrip(t *testing.T) {
	raw := `[{"type":"audio","audio":{"data":"xyz"}}]`
	var content chat.Content
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(content.Parts) != 1 || content.Parts[0].Raw == nil {
		t.Fatalf("parts = %+v", content.Parts)
	}
	got, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(got) != raw {
		t.Fatalf("round trip = %s", got)
	}
}

func TestStopUnion(t *testing.T) {
	one, _ := json.Marshal(chat.Stop{"END"})
	if string(one) != `"END"` {
		t.Errorf("single = %s", one)
	}
	many, _ := json.Marshal(chat.Stop{"a", "b"})
	if string(many) != `["a","b"]` {
		t.Errorf("many = %s", many)
	}
	var decoded chat.Stop
	if err := json.Unmarshal([]byte(`"stop"`), &decoded); err != nil || len(decoded) != 1 {
		t.Errorf("decode string = %v, %v", decoded, err)
	}
}

func TestToolChoiceUnion(t *testing.T) {
	auto, _ := json.Marshal(chat.AutoToolChoice())
	if string(auto) != `"auto"` {
		t.Errorf("auto = %s", auto)
	}
	named, _ := json.Marshal(chat.FunctionToolChoice("get_weather"))
	if string(named) != `{"type":"function","function":{"name":"get_weather"}}` {
		t.Errorf("named = %s", named)
	}
	var decoded chat.ToolChoice
	if err := json.Unmarshal(named, &decoded); err != nil || decoded.Function != "get_weather" {
		t.Errorf("decode named = %+v, %v", decoded, err)
	}
}

func TestContentPartTextAlwaysPresent(t *testing.T) {
	got, err := json.Marshal(chat.PartsContent(chat.TextPart("")))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(got) != `[{"type":"text","text":""}]` {
		t.Fatalf("empty text part = %s", got)
	}

	got, err = json.Marshal(chat.PartsContent(chat.ImagePart("https://example.com/x.png", "")))
	if err != nil {
		t.Fatalf("marshal image: %v", err)
	}
	var parts []map[string]json.RawMessage
	if err := json.Unmarshal(got, &parts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := parts[0]["text"]; ok {
		t.Fatalf("image part must not carry text: %s", got)
	}
}

func TestContentPartKeepsExtraOnKnownType(t *testing.T) {
	raw := `{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}}`
	var part chat.ContentPart
	if err := json.Unmarshal([]byte(raw), &part); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if part.Raw != nil {
		t.Fatalf("known type must not keep Raw")
	}
	if string(part.Extra["cache_control"]) != `{"type":"ephemeral"}` {
		t.Fatalf("extra = %v", part.Extra)
	}
	part.Text = "bye"
	got, err := json.Marshal(part)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(got, &fields); err != nil {
		t.Fatalf("decode re-marshal: %v", err)
	}
	if string(fields["text"]) != `"bye"` {
		t.Fatalf("text = %s", fields["text"])
	}
	if string(fields["cache_control"]) != `{"type":"ephemeral"}` {
		t.Fatalf("cache_control = %s", fields["cache_control"])
	}
}

func TestStopNullRoundTrip(t *testing.T) {
	var stop chat.Stop
	if err := json.Unmarshal([]byte("null"), &stop); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if stop != nil {
		t.Fatalf("stop = %v", stop)
	}
	got, err := json.Marshal(stop)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(got) != `null` {
		t.Fatalf("null stop = %s", got)
	}

	var req chat.Request
	if err := json.Unmarshal([]byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"stop":null}`), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if _, ok := fields["stop"]; ok {
		t.Fatalf("stop must be omitted on re-marshal: %s", body)
	}
}

func TestToolChoiceNullRoundTrip(t *testing.T) {
	var choice chat.ToolChoice
	if err := json.Unmarshal([]byte("null"), &choice); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if !choice.Null {
		t.Fatalf("choice = %+v", choice)
	}
	got, err := json.Marshal(choice)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(got) != `null` {
		t.Fatalf("null choice = %s", got)
	}

	req := &chat.Request{Model: "m", Messages: []chat.Message{chat.UserMessage("hi")}, ToolChoice: &choice}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
