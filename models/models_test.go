package models

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chimixiong/deepseek-sdk-go"
)

const listFixture = `{
  "object": "list",
  "data": [
    {
      "id": "deepseek-flash",
      "object": "model",
      "owned_by": "deepseek",
      "name": "DeepSeek-V4.1-Flash",
      "context_window": 1048576,
      "max_output_tokens": 393216,
      "input_modalities": ["text", "image"],
      "output_modalities": ["text"],
      "effort": {
        "supported_levels": ["low", "high", "max"],
        "default_level": "high"
      },
      "api_capabilities": {
        "anthropic_messages": {
          "system_prompt_update": "in-history"
        }
      }
    },
    {
      "id": "deepseek-v4-pro",
      "object": "model",
      "owned_by": "deepseek",
      "name": "DeepSeek-V4-Pro",
      "context_window": 1048576,
      "max_output_tokens": 393216,
      "input_modalities": ["text"],
      "output_modalities": ["text"],
      "effort": {
        "supported_levels": ["low", "high", "max"],
        "default_level": "high"
      },
      "api_capabilities": {
        "anthropic_messages": {
          "system_prompt_update": "leading-only"
        }
      }
    }
  ]
}`

func TestList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, listFixture)
	}))
	defer server.Close()

	client := New(deepseek.New(deepseek.Config{APIKey: "test-key", BaseURL: server.URL}))
	list, err := client.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list.Object != "list" || len(list.Data) != 2 {
		t.Fatalf("list = %#v", list)
	}

	flash := list.Data[0]
	if flash.ID != "deepseek-flash" || flash.Object != "model" || flash.OwnedBy != "deepseek" {
		t.Errorf("identity = %#v", flash)
	}
	if flash.Name != "DeepSeek-V4.1-Flash" {
		t.Errorf("name = %q", flash.Name)
	}
	if flash.ContextWindow == nil || *flash.ContextWindow != 1048576 ||
		flash.MaxOutputTokens == nil || *flash.MaxOutputTokens != 393216 {
		t.Errorf("limits = %v %v", flash.ContextWindow, flash.MaxOutputTokens)
	}
	if len(flash.InputModalities) != 2 || flash.InputModalities[0] != "text" || flash.InputModalities[1] != "image" {
		t.Errorf("input modalities = %v", flash.InputModalities)
	}
	if len(flash.OutputModalities) != 1 || flash.OutputModalities[0] != "text" {
		t.Errorf("output modalities = %v", flash.OutputModalities)
	}
	if flash.Effort == nil || flash.Effort.DefaultLevel == nil || *flash.Effort.DefaultLevel != "high" {
		t.Errorf("effort = %#v", flash.Effort)
	}
	if flash.Effort != nil && (len(flash.Effort.SupportedLevels) != 3 || flash.Effort.SupportedLevels[2] != "max") {
		t.Errorf("supported levels = %v", flash.Effort.SupportedLevels)
	}
	if flash.APICapabilities == nil || flash.APICapabilities.AnthropicMessages == nil ||
		flash.APICapabilities.AnthropicMessages.SystemPromptUpdate != SystemPromptUpdateInHistory {
		t.Errorf("capabilities = %#v", flash.APICapabilities)
	}
	if pro := list.Data[1]; pro.APICapabilities == nil ||
		pro.APICapabilities.AnthropicMessages.SystemPromptUpdate != SystemPromptUpdateLeadingOnly {
		t.Errorf("pro capabilities = %#v", pro.APICapabilities)
	}
}

func TestModelOptionalMetadataAbsent(t *testing.T) {
	var model Model
	if err := json.Unmarshal([]byte(`{"id":"m","object":"model","owned_by":"o"}`), &model); err != nil {
		t.Fatal(err)
	}
	if model.ContextWindow != nil || model.MaxOutputTokens != nil || model.Effort != nil ||
		model.APICapabilities != nil || model.InputModalities != nil {
		t.Errorf("optional metadata = %#v", model)
	}
}

func TestNewNilClient(t *testing.T) {
	if client := New(nil); client == nil || client.api == nil {
		t.Fatal("New(nil) did not install a default transport client")
	}
}

func TestNilClientList(t *testing.T) {
	var client *Client
	if _, err := client.List(context.Background()); err == nil {
		t.Fatal("List on a nil client returned no error")
	}
}
