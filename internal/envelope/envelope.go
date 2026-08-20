package envelope

import (
	"encoding/json"
	"io"
)

func Print(w io.Writer, e any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(e)
}
