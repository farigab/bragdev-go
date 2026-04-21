// Package httpresp provides helpers to write HTTP responses in JSON.
package httpresp

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents a simple JSON error payload.
type ErrorResponse struct {
	Error string `json:"error"`
}

// JSONError writes a consistent JSON error response with given status.
func JSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
