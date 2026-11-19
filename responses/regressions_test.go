package responses_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go/responses"
)

func TestContentPartRoundTripKeepsEdits(t *testing.T) {
	var part responses.ContentPart
	if err := json.Unmarshal([]byte(`{"type":"input_text","text":"original","unknown":1}`), &part); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if part.Text != "original" || part.Type != responses.PartInputText {
		t.Fatalf("part = %+v", part)
	}
	if len(part.Extra) != 1 || string(part.Extra["unknown"]) != "1" {
		t.Errorf("extra = %v, want only unknown", part.Extra)
	}

	part.Text = "changed"
	encoded, err := json.Marshal(part)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("decode marshalled part %s: %v", encoded, err)
	}
	if got := rawString(t, object["text"]); got != "changed" {
		t.Errorf("text = %q, want changed", got)
	}
	if got := rawString(t, object["type"]); got != "input_text" {
		t.Errorf("type = %q", got)
	}
	if got := string(object["unknown"]); got != "1" {
		t.Errorf("unknown = %s", got)
	}
}

func TestContentPartUnknownTypeRoundTrip(t *testing.T) {
	raw := `{"type":"future_part","foo":{"a":1}}`
	var part responses.ContentPart
	if err := json.Unmarshal([]byte(raw), &part); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	encoded, err := json.Marshal(part)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(encoded) != raw {
		t.Errorf("round trip = %s, want %s", encoded, raw)
	}
}

func TestContentPartExtraDoesNotShadowModeledFields(t *testing.T) {
	part := responses.InputTextPart("kept")
	part.Extra = map[string]json.RawMessage{"unknown": json.RawMessage(`true`)}
	encoded, err := json.Marshal(part)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("decode %s: %v", encoded, err)
	}
	if got := rawString(t, object["text"]); got != "kept" {
		t.Errorf("text = %q", got)
	}
	if got := rawString(t, object["type"]); got != "input_text" {
		t.Errorf("type = %q", got)
	}
	if got := string(object["unknown"]); got != "true" {
		t.Errorf("unknown = %s", got)
	}
}

func TestContentPartEmptyTextKeepsRequiredField(t *testing.T) {
	for _, part := range []responses.ContentPart{
		responses.InputTextPart(""),
		responses.OutputTextPart(""),
		responses.ReasoningTextPart(""),
	} {
		encoded, err := json.Marshal(part)
		if err != nil {
			t.Fatalf("Marshal %s: %v", part.Type, err)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &object); err != nil {
			t.Fatalf("decode %s: %v", encoded, err)
		}
		text, ok := object["text"]
		if !ok {
			t.Errorf("%s marshalled as %s, want text field", part.Type, encoded)
			continue
		}
		if string(text) != `""` {
			t.Errorf("%s text = %s, want empty string", part.Type, text)
		}
	}

	encoded, err := json.Marshal(responses.InputImagePart("https://example.com/a.png", ""))
	if err != nil {
		t.Fatalf("Marshal image: %v", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("decode image %s: %v", encoded, err)
	}
	if _, ok := object["text"]; ok {
		t.Errorf("input_image marshalled as %s, want no text field", encoded)
	}
}

func TestToolChoiceNullRoundTrip(t *testing.T) {
	var choice responses.ToolChoice
	if err := json.Unmarshal([]byte("null"), &choice); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !choice.Null {
		t.Fatalf("choice = %+v, want Null", choice)
	}
	encoded, err := json.Marshal(choice)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(encoded) != "null" {
		t.Errorf("Marshal = %s, want null", encoded)
	}
	req := &responses.Request{
		Model:      "deepseek-flash",
		Input:      responses.StringInput("hi"),
		ToolChoice: &choice,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestRequestToolChoiceNullStaysOmitted(t *testing.T) {
	var req responses.Request
	if err := json.Unmarshal([]byte(`{"model":"deepseek-flash","input":"hi","tool_choice":null}`), &req); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if req.ToolChoice != nil {
		t.Fatalf("tool choice = %+v, want nil", req.ToolChoice)
	}
	if err := req.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestTopPAcceptsZero(t *testing.T) {
	req := &responses.Request{
		Model: "deepseek-flash",
		Input: responses.StringInput("hi"),
		TopP:  new(0.0),
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate top_p 0: %v", err)
	}
	req.TopP = new(1.0)
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate top_p 1: %v", err)
	}
	req.TopP = new(1.1)
	if err := req.Validate(); err == nil {
		t.Fatal("Validate accepted top_p 1.1")
	}
	req.TopP = new(-0.1)
	if err := req.Validate(); err == nil {
		t.Fatal("Validate accepted top_p -0.1")
	}
}

func TestNewNilClient(t *testing.T) {
	if client := responses.New(nil); client == nil {
		t.Fatal("New(nil) = nil")
	}
}

func TestNilClientMethods(t *testing.T) {
	var client *responses.Client
	if _, err := client.Create(context.Background(), &responses.Request{
		Model: "deepseek-flash",
		Input: responses.StringInput("hi"),
	}); err == nil {
		t.Fatal("Create on a nil client returned no error")
	}
	var got error
	for _, err := range client.CreateStream(context.Background(), &responses.Request{
		Model: "deepseek-flash",
		Input: responses.StringInput("hi"),
	}) {
		if err != nil {
			got = err
		}
	}
	if got == nil {
		t.Fatal("CreateStream on a nil client returned no error")
	}
}
