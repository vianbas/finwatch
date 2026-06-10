// Package web holds HTTP helpers shared across the API surface: JSON responses
// and the canonical error envelope. Feature modules use these so every endpoint
// returns the same shape, matching contracts/openapi.yaml.
package web

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the canonical API error envelope. It mirrors the Error
// schema in contracts/openapi.yaml.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries a stable machine-readable code and a human-readable
// message safe to display.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON serialises v as JSON with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// The header is already committed; an encode error cannot be recovered here.
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes the canonical error envelope.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{Error: ErrorDetail{Code: code, Message: message}})
}
