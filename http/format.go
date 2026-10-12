package fbhttp

import (
	"errors"
	"fmt"
	"go/format"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

// maxFormatSize bounds the source accepted by the formatting endpoint.
const maxFormatSize = 5 << 20

// formatGo formats Go source the way gofmt does.
func formatGo(src []byte) ([]byte, error) {
	return format.Source(src)
}

// formatHandler formats the text in the request body for the given language,
// for the editor's "Format" action. Prettier and Ruff run in the browser; Go is
// formatted here because the standard library already includes gofmt.
var formatHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var formatter func([]byte) ([]byte, error)
	switch mux.Vars(r)["lang"] {
	case "go":
		formatter = formatGo
	default:
		return http.StatusNotFound, nil
	}

	if r.Body == nil {
		return http.StatusBadRequest, errors.New("empty request")
	}

	src, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxFormatSize))
	if err != nil {
		return http.StatusRequestEntityTooLarge, fmt.Errorf("source is larger than %d bytes", maxFormatSize)
	}

	out, err := formatter(src)
	if err != nil {
		return http.StatusBadRequest, err
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write(out); err != nil {
		return 0, err
	}

	return 0, nil
})
