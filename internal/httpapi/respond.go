package httpapi

import (
	"encoding/json"
	"net/http"
)

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// ptr returns a pointer to v, for building pointer-typed struct literal
// fields (e.g. audit.Event.EntityType) from a value in one expression.
func ptr[T any](v T) *T {
	return &v
}

// nonNilSlice returns s, or a non-nil empty slice if s is nil. A JSON body
// that omits an array field (or sets it to `null`) decodes to a nil Go
// slice; pgx encodes that as SQL NULL rather than an empty array, which
// violates the `not null default '{}'` this app's text[] columns rely on.
func nonNilSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
