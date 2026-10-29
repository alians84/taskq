package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON_EncodeFailureReturns500(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]any{"unencodable": make(chan int)})
	assertJSON(t, rec, http.StatusInternalServerError, `{"error":"internal error"}`)
}
