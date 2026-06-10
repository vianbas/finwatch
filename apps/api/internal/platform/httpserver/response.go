package httpserver

import (
	"encoding/json"
	"net/http"
)

// errorBody is the canonical API error envelope. It mirrors the Error schema
// declared in contracts/openapi.yaml so the contract and implementation stay
// aligned.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON serialises v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// Encoding a well-typed value to an already-committed header cannot
	// meaningfully recover; the error is intentionally ignored.
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes the canonical error envelope.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}
