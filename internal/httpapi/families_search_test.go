package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleSearchFamilies_Success(t *testing.T) {
	var captured db.SearchFamiliesParams
	q := &dbfake.Querier{
		SearchFamiliesFunc: func(ctx context.Context, arg db.SearchFamiliesParams) ([]db.Family, error) {
			captured = arg
			return []db.Family{{ID: 1}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/families/search?q=maria&email=m%40example.edu&city=Boulder&limit=5", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.NameQuery == nil || *captured.NameQuery != "maria" {
		t.Errorf("NameQuery = %v, want maria", captured.NameQuery)
	}
	if captured.Email == nil || *captured.Email != "m@example.edu" {
		t.Errorf("Email = %v, want m@example.edu", captured.Email)
	}
	if captured.City == nil || *captured.City != "Boulder" {
		t.Errorf("City = %v, want Boulder", captured.City)
	}
	if captured.LimitCount != 5 {
		t.Errorf("LimitCount = %d, want 5", captured.LimitCount)
	}
	got := decodeBody[[]familyResponse](t, rec)
	if len(got) != 1 {
		t.Errorf("got %d results, want 1", len(got))
	}
}

func TestHandleSearchFamilies_DoesNotCollideWithGetFamily(t *testing.T) {
	q := &dbfake.Querier{
		SearchFamiliesFunc: func(ctx context.Context, arg db.SearchFamiliesParams) ([]db.Family, error) {
			return []db.Family{}, nil
		},
		GetFamilyByIDFunc: func(ctx context.Context, id int64) (db.Family, error) {
			t.Fatal("GetFamilyByID should not be called for /families/search")
			return db.Family{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/families/search", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}
