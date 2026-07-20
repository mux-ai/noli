package protocol

import (
	"encoding/json"
	"io"
)

// Write encodes the response as compact JSON with exactly one trailing
// newline and no HTML escaping. JSON output goes only to the given writer
// (stdout in the CLI); it carries no colors, banners, or progress text.
func Write(w io.Writer, response Response) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(response)
}
