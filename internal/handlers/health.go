package handlers

import "net/http"

// HealthHandler responds with a minimal JSON health status.
// HealthHandler writes a 200 OK with {"status":"ok"}.
func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
