package deepseek

import "encoding/json"

// MarshalWithExtra adds top-level fields to the JSON object produced for v.
// Protocol requests use it to accept fields the SDK does not model yet, for
// example DeepSeek extensions added after a release.
func MarshalWithExtra(v any, extra map[string]json.RawMessage) ([]byte, error) {
	base, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(extra) == 0 {
		return base, nil
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
