package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleListRoles_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListRolesFunc: func(ctx context.Context) ([]db.Role, error) {
			return []db.Role{
				{ID: 1, Name: "staff", Description: "Day-to-day scheduling and participant search"},
				{ID: 2, Name: "coordinator", Description: "Release/hold overrides and reporting"},
				{ID: 3, Name: "admin", Description: "Full lab configuration access"},
			}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/roles", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]roleResponse](t, rec)
	if len(got) != 3 || got[0].Name != "staff" {
		t.Errorf("roles = %+v", got)
	}
}

func TestHandleListRoles_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodGet, "/roles", nil, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
