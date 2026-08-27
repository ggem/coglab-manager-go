package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryString(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?q=jon&empty=", nil)
	if got := queryString(r, "q"); got == nil || *got != "jon" {
		t.Errorf("queryString(q) = %v, want jon", got)
	}
	if got := queryString(r, "empty"); got != nil {
		t.Errorf("queryString(empty) = %v, want nil", got)
	}
	if got := queryString(r, "missing"); got != nil {
		t.Errorf("queryString(missing) = %v, want nil", got)
	}
}

func TestQueryBool(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?t=true&f=false&bad=nope", nil)

	if v, ok := queryBool(httptest.NewRecorder(), r, "t", false); !ok || !v {
		t.Errorf("queryBool(t) = %v, %v, want true, true", v, ok)
	}
	if v, ok := queryBool(httptest.NewRecorder(), r, "f", true); !ok || v {
		t.Errorf("queryBool(f) = %v, %v, want false, true", v, ok)
	}
	if v, ok := queryBool(httptest.NewRecorder(), r, "missing", true); !ok || !v {
		t.Errorf("queryBool(missing) = %v, %v, want default true, true", v, ok)
	}
	rec := httptest.NewRecorder()
	if _, ok := queryBool(rec, r, "bad", false); ok {
		t.Error("queryBool(bad) ok = true, want false")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestQueryBoolPtr(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?t=true&bad=nope", nil)

	if v, ok := queryBoolPtr(httptest.NewRecorder(), r, "t"); !ok || v == nil || !*v {
		t.Errorf("queryBoolPtr(t) = %v, %v", v, ok)
	}
	if v, ok := queryBoolPtr(httptest.NewRecorder(), r, "missing"); !ok || v != nil {
		t.Errorf("queryBoolPtr(missing) = %v, %v, want nil, true", v, ok)
	}
	if _, ok := queryBoolPtr(httptest.NewRecorder(), r, "bad"); ok {
		t.Error("queryBoolPtr(bad) ok = true, want false")
	}
}

func TestQueryInt64Ptr(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?id=42&bad=nope", nil)

	if v, ok := queryInt64Ptr(httptest.NewRecorder(), r, "id"); !ok || v == nil || *v != 42 {
		t.Errorf("queryInt64Ptr(id) = %v, %v, want 42, true", v, ok)
	}
	if v, ok := queryInt64Ptr(httptest.NewRecorder(), r, "missing"); !ok || v != nil {
		t.Errorf("queryInt64Ptr(missing) = %v, %v, want nil, true", v, ok)
	}
	if _, ok := queryInt64Ptr(httptest.NewRecorder(), r, "bad"); ok {
		t.Error("queryInt64Ptr(bad) ok = true, want false")
	}
}

func TestQueryLimit(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=10", nil)
	if v, ok := queryLimit(httptest.NewRecorder(), r, 50, 200); !ok || v != 10 {
		t.Errorf("queryLimit(10) = %v, %v, want 10, true", v, ok)
	}

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	if v, ok := queryLimit(httptest.NewRecorder(), r, 50, 200); !ok || v != 50 {
		t.Errorf("queryLimit(missing) = %v, %v, want default 50, true", v, ok)
	}

	r = httptest.NewRequest(http.MethodGet, "/?limit=9999", nil)
	if v, ok := queryLimit(httptest.NewRecorder(), r, 50, 200); !ok || v != 200 {
		t.Errorf("queryLimit(9999) = %v, %v, want capped at 200, true", v, ok)
	}

	for _, bad := range []string{"0", "-5", "nope"} {
		r = httptest.NewRequest(http.MethodGet, "/?limit="+bad, nil)
		rec := httptest.NewRecorder()
		if _, ok := queryLimit(rec, r, 50, 200); ok {
			t.Errorf("queryLimit(%q) ok = true, want false", bad)
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("queryLimit(%q) status = %d, want %d", bad, rec.Code, http.StatusBadRequest)
		}
	}
}
