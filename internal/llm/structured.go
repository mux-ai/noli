package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// DecodeStrictJSON decodes exactly one JSON value and rejects unknown fields.
// Maps (including arbitrary concept attributes) remain unrestricted by design.
func DecodeStrictJSON(data []byte, output any) error {
	if output == nil {
		return fmt.Errorf("decode structured JSON: output must not be nil")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("decode structured JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode structured JSON: multiple JSON values")
		}
		return fmt.Errorf("decode structured JSON trailing data: %w", err)
	}
	return nil
}
