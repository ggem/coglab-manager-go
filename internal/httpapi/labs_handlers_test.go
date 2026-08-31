package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleListMyLabs_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListLabsForUserFunc: func(ctx context.Context, userID int64) ([]db.Lab, error) {
			return []db.Lab{
				{ID: 1, Name: "Cognitive Development Center", ShortName: "CDC"},
				{ID: 2, Name: "Developmental & Attachment Studies", ShortName: "DACS"},
			}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]labResponse](t, rec)
	if len(got) != 2 || got[0].ShortName != "CDC" || got[1].ShortName != "DACS" {
		t.Errorf("labs = %+v, want CDC then DACS", got)
	}
}

func TestHandleListMyLabs_Empty(t *testing.T) {
	q := &dbfake.Querier{
		ListLabsForUserFunc: func(ctx context.Context, userID int64) ([]db.Lab, error) {
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]labResponse](t, rec)
	if len(got) != 0 {
		t.Errorf("labs = %+v, want none", got)
	}
}

func TestHandleListMyLabs_UnexpectedDBError(t *testing.T) {
	// Anything writeDBError doesn't recognize (e.g. a dropped connection)
	// must come back as a 500 without leaking the underlying error.
	q := &dbfake.Querier{
		ListLabsForUserFunc: func(ctx context.Context, userID int64) ([]db.Lab, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	got := decodeBody[errorResponse](t, rec)
	if got.Error != "internal error" {
		t.Errorf("error body = %q, want a generic message that doesn't leak the underlying error", got.Error)
	}
}

func TestHandleListMyLabs_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodGet, "/labs", nil, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
