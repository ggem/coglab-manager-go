package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleSearchChildren_Success(t *testing.T) {
	var captured db.SearchChildrenParams
	q := &dbfake.Querier{
		SearchChildrenFunc: func(ctx context.Context, arg db.SearchChildrenParams) ([]db.Child, error) {
			captured = arg
			return []db.Child{{ID: 1}, {ID: 2}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/children/search?q=jon&sex=female&twin=true&language=spanish&family_id=3&min_birth_date=2023-01-01&max_birth_date=2023-12-31&include_deactivated=true&limit=10", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.NameQuery == nil || *captured.NameQuery != "jon" {
		t.Errorf("NameQuery = %v, want jon", captured.NameQuery)
	}
	if captured.Sex == nil || *captured.Sex != "female" {
		t.Errorf("Sex = %v, want female", captured.Sex)
	}
	if captured.Twin == nil || !*captured.Twin {
		t.Errorf("Twin = %v, want true", captured.Twin)
	}
	if captured.Language == nil || *captured.Language != "spanish" {
		t.Errorf("Language = %v, want spanish", captured.Language)
	}
	if captured.FamilyID == nil || *captured.FamilyID != 3 {
		t.Errorf("FamilyID = %v, want 3", captured.FamilyID)
	}
	if !captured.MinBirthDate.Valid || captured.MinBirthDate.Time.Format(dateLayout) != "2023-01-01" {
		t.Errorf("MinBirthDate = %+v, want 2023-01-01", captured.MinBirthDate)
	}
	if !captured.IncludeDeactivated {
		t.Error("IncludeDeactivated = false, want true")
	}
	if captured.LimitCount != 10 {
		t.Errorf("LimitCount = %d, want 10", captured.LimitCount)
	}
	got := decodeBody[[]childResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("got %d results, want 2", len(got))
	}
}

func TestHandleSearchChildren_NoFilters(t *testing.T) {
	var captured db.SearchChildrenParams
	q := &dbfake.Querier{
		SearchChildrenFunc: func(ctx context.Context, arg db.SearchChildrenParams) ([]db.Child, error) {
			captured = arg
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/children/search", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.NameQuery != nil {
		t.Errorf("NameQuery = %v, want nil", captured.NameQuery)
	}
	if captured.LimitCount != defaultSearchLimit {
		t.Errorf("LimitCount = %d, want default %d", captured.LimitCount, defaultSearchLimit)
	}
	if captured.IncludeDeactivated {
		t.Error("IncludeDeactivated = true, want false by default")
	}
}

func TestHandleSearchChildren_InvalidDate(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/children/search?min_birth_date=not-a-date", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// The search route sits alongside /children/{childID}; this confirms
// "search" is matched as the literal route rather than being swallowed as
// a childID path parameter (which would 400 out of idParam instead).
func TestHandleSearchChildren_DoesNotCollideWithGetChild(t *testing.T) {
	q := &dbfake.Querier{
		SearchChildrenFunc: func(ctx context.Context, arg db.SearchChildrenParams) ([]db.Child, error) {
			return []db.Child{}, nil
		},
		GetChildByIDFunc: func(ctx context.Context, id int64) (db.Child, error) {
			t.Fatal("GetChildByID should not be called for /children/search")
			return db.Child{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/children/search", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}
