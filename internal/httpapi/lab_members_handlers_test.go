package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleListLabMembers_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListLabMembersFunc: func(ctx context.Context, labID int64) ([]db.User, error) {
			return []db.User{
				{ID: 1, FirstName: "Pat", LastName: "Lee"},
				{ID: 2, FirstName: "Sam", LastName: "Rivera"},
			}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/members", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]trainedMemberResponse](t, rec)
	if len(got) != 2 || got[0].FirstName != "Pat" || got[1].FirstName != "Sam" {
		t.Errorf("members = %+v, want Pat then Sam", got)
	}
}

func TestHandleListLabMembers_Empty(t *testing.T) {
	q := &dbfake.Querier{
		ListLabMembersFunc: func(ctx context.Context, labID int64) ([]db.User, error) {
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/members", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]trainedMemberResponse](t, rec)
	if len(got) != 0 {
		t.Errorf("members = %+v, want none", got)
	}
}

func TestHandleListLabMembers_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		ListLabMembersFunc: func(ctx context.Context, labID int64) ([]db.User, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/members", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	got := decodeBody[errorResponse](t, rec)
	if got.Error != "internal error" {
		t.Errorf("error body = %q, want a generic message that doesn't leak the underlying error", got.Error)
	}
}

func TestHandleListLabMembers_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodGet, "/labs/9/members", nil, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
