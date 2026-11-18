package messages

import (
	"encoding/json"
	"testing"
)

func TestBlockPreservesUnknownFields(t *testing.T) {
	data := []byte(`{"type":"text","text":"hi","future_field":{"x":1}}`)
	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	encoded, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode round trip: %v", err)
	}
	if string(fields["text"]) != `"hi"` {
		t.Fatalf("text = %s", fields["text"])
	}
	if string(fields["future_field"]) != `{"x":1}` {
		t.Fatalf("future_field = %s", fields["future_field"])
	}
}

func TestBlockDropsRemovedModelledField(t *testing.T) {
	var block Block
	if err := json.Unmarshal([]byte(`{"type":"document","source":{"type":"text","data":"x"},"title":"old"}`), &block); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	block.Title = nil
	encoded, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode round trip: %v", err)
	}
	if _, ok := fields["title"]; ok {
		t.Fatalf("title = %s, want it removed", fields["title"])
	}
}
