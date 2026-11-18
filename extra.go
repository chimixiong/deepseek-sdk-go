package deepseek

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// MarshalWithExtra adds top-level fields to the JSON object produced for v.
// Protocol requests use it to accept fields the SDK does not model yet, for
// example DeepSeek extensions added after a release. An Extra key that names a
// modelled field is rejected: silently overwriting it would bypass Validate and
// the endpoint's forced fields.
func MarshalWithExtra(v any, extra map[string]json.RawMessage) ([]byte, error) {
	if len(extra) == 0 {
		return json.Marshal(v)
	}
	modelled := jsonFieldNames(v)
	for key := range extra {
		if modelled[key] {
			return nil, fmt.Errorf("deepseek: Extra field %q duplicates a modelled field", key)
		}
	}
	base, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]json.RawMessage, len(extra))
	if string(base) != "null" {
		if err := json.Unmarshal(base, &fields); err != nil {
			return nil, err
		}
	}
	for key, value := range extra {
		fields[key] = value
	}
	return json.Marshal(fields)
}

// jsonFieldNames returns every field name the struct can encode, including
// fields omitted by omitempty. Extra must not target any of them.
func jsonFieldNames(v any) map[string]bool {
	t := reflect.TypeOf(v)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	names := make(map[string]bool, t.NumField())
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		names[name] = true
	}
	return names
}
