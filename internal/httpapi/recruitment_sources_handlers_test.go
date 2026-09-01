package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleListRecruitmentSources_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListActiveRecruitmentSourcesFunc: func(ctx context.Context) ([]db.RecruitmentSource, error) {
			return []db.RecruitmentSource{
				{ID: 1, Name: "Birth records", Active: true},
				{ID: 2, Name: "Referral", Active: true},
			}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/recruitment-sources", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]recruitmentSourceResponse](t, rec)
	if len(got) != 2 || got[0].Name != "Birth records" || got[1].Name != "Referral" {
		t.Errorf("sources = %+v, want Birth records then Referral", got)
	}
}

func TestHandleListRecruitmentSources_Empty(t *testing.T) {
	q := &dbfake.Querier{
		ListActiveRecruitmentSourcesFunc: func(ctx context.Context) ([]db.RecruitmentSource, error) {
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/recruitment-sources", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]recruitmentSourceResponse](t, rec)
	if len(got) != 0 {
		t.Errorf("sources = %+v, want none", got)
	}
}

func TestHandleListRecruitmentSources_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		ListActiveRecruitmentSourcesFunc: func(ctx context.Context) ([]db.RecruitmentSource, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/recruitment-sources", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	got := decodeBody[errorResponse](t, rec)
	if got.Error != "internal error" {
		t.Errorf("error body = %q, want a generic message that doesn't leak the underlying error", got.Error)
	}
}

func TestHandleListRecruitmentSources_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodGet, "/recruitment-sources", nil, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateRecruitmentSource_Success(t *testing.T) {
	var captured string
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateRecruitmentSourceFunc: func(ctx context.Context, name string) (db.RecruitmentSource, error) {
			captured = name
			return db.RecruitmentSource{ID: 9, Name: name, Active: true}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/recruitment-sources", cookie, recruitmentSourceRequest{Name: "Flyer"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured != "Flyer" {
		t.Errorf("CreateRecruitmentSource name = %q, want %q", captured, "Flyer")
	}
	got := decodeBody[recruitmentSourceResponse](t, rec)
	if got.ID != 9 {
		t.Errorf("response ID = %d, want 9", got.ID)
	}
	if capturedAudit.Action != ActionRecruitmentSourceCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionRecruitmentSourceCreated)
	}
}

func TestHandleCreateRecruitmentSource_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		CreateRecruitmentSourceFunc: func(ctx context.Context, name string) (db.RecruitmentSource, error) {
			return db.RecruitmentSource{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/recruitment-sources", cookie, recruitmentSourceRequest{Name: "Flyer"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	got := decodeBody[errorResponse](t, rec)
	if got.Error != "internal error" {
		t.Errorf("error body = %q, want a generic message that doesn't leak the underlying error", got.Error)
	}
}

func TestHandleCreateRecruitmentSource_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/recruitment-sources", nil, recruitmentSourceRequest{Name: "Flyer"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
