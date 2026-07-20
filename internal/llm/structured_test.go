package llm

import "testing"

func TestDecodeStrictJSONRejectsUnknownAndTrailingData(t *testing.T) {
	type output struct {
		Name string `json:"name"`
	}
	var decoded output
	if err := DecodeStrictJSON([]byte(`{"name":"ok","extra":true}`), &decoded); err == nil {
		t.Fatal("expected unknown field error")
	}
	if err := DecodeStrictJSON([]byte(`{"name":"ok"} {"name":"two"}`), &decoded); err == nil {
		t.Fatal("expected multiple value error")
	}
}
