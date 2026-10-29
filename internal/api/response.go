package api

import (
	"encoding/json"
	"net/http"
)

// internalErrorBody is sent whenever a response cannot be produced as intended.
const internalErrorBody = `{"error":"internal error"}` + "\n"

// writeJSON writes v as a JSON response with the given status code.
// v is encoded before any header is sent, so an encoding failure still
// yields a proper 500 instead of a 200 with a broken body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(internalErrorBody))
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}
