package httpapi

import (
	"net/http"
	"strconv"
)

// Default and maximum "limit" for search endpoints.
const (
	defaultSearchLimit int32 = 50
	maxSearchLimit     int32 = 200
)

// queryString returns the named query parameter, or nil if it's absent or
// empty -- there's no meaningful distinction between "not provided" and
// "provided as an empty string" for an optional search filter.
func queryString(r *http.Request, name string) *string {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil
	}
	return &v
}

// queryBool parses the named query parameter as a bool, returning def if
// it's absent. Writes a 400 and returns ok=false if it's present but not a
// valid bool.
func queryBool(w http.ResponseWriter, r *http.Request, name string, def bool) (val bool, ok bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return false, false
	}
	return b, true
}

// queryBoolPtr is queryBool for an optional (nullable) filter rather than
// one with a default.
func queryBoolPtr(w http.ResponseWriter, r *http.Request, name string) (val *bool, ok bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return nil, false
	}
	return &b, true
}

// queryInt64Ptr parses the named query parameter as an int64, or nil if
// it's absent. Writes a 400 and returns ok=false if it's present but not a
// valid integer.
func queryInt64Ptr(w http.ResponseWriter, r *http.Request, name string) (val *int64, ok bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, true
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return nil, false
	}
	return &n, true
}

// queryLimit parses the "limit" query parameter, defaulting to def and
// capping at max. Writes a 400 and returns ok=false if it's present but
// not a positive integer.
func queryLimit(w http.ResponseWriter, r *http.Request, def, max int32) (val int32, ok bool) {
	v := r.URL.Query().Get("limit")
	if v == "" {
		return def, true
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil || n <= 0 {
		writeError(w, http.StatusBadRequest, "invalid limit")
		return 0, false
	}
	if int32(n) > max {
		return max, true
	}
	return int32(n), true
}
