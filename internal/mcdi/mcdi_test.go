package mcdi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGenderFor(t *testing.T) {
	tests := []struct {
		sex  string
		want string
	}{
		{"male", "male"},
		{"female", "female"},
		{"unknown", "other"},
		{"", "other"},
	}
	for _, tt := range tests {
		if got := GenderFor(tt.sex); got != tt.want {
			t.Errorf("GenderFor(%q) = %q, want %q", tt.sex, got, tt.want)
		}
	}
}

func TestAPIClient_RequestSurvey_Success(t *testing.T) {
	var gotQuery url.Values
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"msg": "success"}`))
	}))
	defer srv.Close()

	c := NewAPIClient(srv.URL, "test-key", "")
	err := c.RequestSurvey(context.Background(), Request{
		ChildName: "Kid Test", ParentEmail: "parent@example.edu",
		Gender: "female", Birthday: "2024-01-15", DatabaseID: 42,
	})
	if err != nil {
		t.Fatalf("RequestSurvey: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	want := map[string]string{
		"api_key": "test-key", "child_name": "Kid Test", "cdi_type": "fullenglishmcdi",
		"parent_email": "parent@example.edu", "format": "standard", "database_id": "42",
		"gender": "female", "birthday": "2024-01-15",
	}
	for k, v := range want {
		if got := gotQuery.Get(k); got != v {
			t.Errorf("query[%q] = %q, want %q", k, got, v)
		}
	}
}

func TestAPIClient_RequestSurvey_ConfigurableTypeParam(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`{"msg": "success"}`))
	}))
	defer srv.Close()

	c := NewAPIClient(srv.URL, "test-key", "mcdi_type")
	if err := c.RequestSurvey(context.Background(), Request{DatabaseID: 1}); err != nil {
		t.Fatalf("RequestSurvey: %v", err)
	}

	if got := gotQuery.Get("mcdi_type"); got != "fullenglishmcdi" {
		t.Errorf("query[mcdi_type] = %q, want fullenglishmcdi", got)
	}
	if gotQuery.Has("cdi_type") {
		t.Error("query has cdi_type set, want only the configured mcdi_type param")
	}
}

func TestAPIClient_RequestSurvey_ErrorField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error": "invalid parent_email"}`))
	}))
	defer srv.Close()

	err := NewAPIClient(srv.URL, "test-key", "").RequestSurvey(context.Background(), Request{DatabaseID: 1})

	if err == nil || !strings.Contains(err.Error(), "invalid parent_email") {
		t.Errorf("err = %v, want it to mention the API's error message", err)
	}
}

func TestAPIClient_RequestSurvey_NonSuccessMsg(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"msg": "pending"}`))
	}))
	defer srv.Close()

	err := NewAPIClient(srv.URL, "test-key", "").RequestSurvey(context.Background(), Request{DatabaseID: 1})

	if err == nil {
		t.Fatal("err = nil, want an error for a non-success msg")
	}
}

func TestAPIClient_RequestSurvey_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
	}))
	defer srv.Close()

	err := NewAPIClient(srv.URL, "test-key", "").RequestSurvey(context.Background(), Request{DatabaseID: 1})

	if err == nil {
		t.Fatal("err = nil, want an error for a 500 response")
	}
}

func TestAPIClient_RequestSurvey_ConnectionError(t *testing.T) {
	c := NewAPIClient("http://127.0.0.1:1", "test-key", "")

	err := c.RequestSurvey(context.Background(), Request{DatabaseID: 1})

	if err == nil {
		t.Fatal("err = nil, want a connection error against an unreachable address")
	}
}
