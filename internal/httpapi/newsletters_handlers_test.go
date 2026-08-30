package httpapi

import (
	"context"
	"encoding/csv"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleCreateNewsletter_Success(t *testing.T) {
	var captured db.CreateNewsletterParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		CreateNewsletterFunc: func(ctx context.Context, arg db.CreateNewsletterParams) (db.Newsletter, error) {
			captured = arg
			return db.Newsletter{ID: 1, LabID: arg.LabID, Name: arg.Name}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/newsletters/", cookie, newsletterRequest{Name: "Spring 2026"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.LabID != 9 || captured.Name != "Spring 2026" {
		t.Errorf("CreateNewsletter params = %+v", captured)
	}
	got := decodeBody[newsletterResponse](t, rec)
	if got.ID != 1 {
		t.Errorf("response ID = %d, want 1", got.ID)
	}
	if capturedAudit.Action != ActionNewsletterCreated {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionNewsletterCreated)
	}
}

func TestHandleCreateNewsletter_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/labs/9/newsletters/", nil, newsletterRequest{})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateNewsletter_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/not-a-number/newsletters/", cookie, newsletterRequest{Name: "X"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateNewsletter_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		CreateNewsletterFunc: func(ctx context.Context, arg db.CreateNewsletterParams) (db.Newsletter, error) {
			return db.Newsletter{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/newsletters/", cookie, newsletterRequest{Name: "X"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleGetNewsletter_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/newsletters/not-a-number/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetNewsletter_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetNewsletterByIDFunc: func(ctx context.Context, id int64) (db.Newsletter, error) {
			return db.Newsletter{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/newsletters/404/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleListNewslettersByLab_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/not-a-number/newsletters/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListNewslettersByLab_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListNewslettersByLabFunc: func(ctx context.Context, labID int64) ([]db.Newsletter, error) {
			return []db.Newsletter{{ID: 1, LabID: labID, Name: "A"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/newsletters/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]newsletterResponse](t, rec)
	if len(got) != 1 || got[0].Name != "A" {
		t.Errorf("response = %+v", got)
	}
}

func TestHandleDeactivateNewsletter_InvalidID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/newsletters/not-a-number/deactivate", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateNewsletter_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetNewsletterByIDFunc: func(ctx context.Context, id int64) (db.Newsletter, error) {
			return db.Newsletter{ID: id, LabID: 1}, nil
		},
		DeactivateNewsletterFunc: func(ctx context.Context, id int64) error {
			return pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/newsletters/404/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateNewsletter_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetNewsletterByIDFunc: func(ctx context.Context, id int64) (db.Newsletter, error) {
			return db.Newsletter{ID: id, LabID: 1}, nil
		},
		DeactivateNewsletterFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/newsletters/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
}

// --- export ---

func TestHandleExportNewsletter_Success(t *testing.T) {
	var captured db.ListEligibleFamiliesForNewsletterParams
	q := &dbfake.Querier{
		ListEligibleFamiliesForNewsletterFunc: func(ctx context.Context, arg db.ListEligibleFamiliesForNewsletterParams) ([]db.ListEligibleFamiliesForNewsletterRow, error) {
			captured = arg
			return []db.ListEligibleFamiliesForNewsletterRow{
				{FamilyID: 1, GuardianID: 10, FirstName: "Jane", LastName: "Doe", Address: "1 Main St", City: "Boulder", State: "CO", Zip: "80301"},
				{FamilyID: 2, GuardianID: 20, FirstName: "John", LastName: "Smith", Address: "2 Elm St", City: "Boulder", State: "CO", Zip: "80302"},
			}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/newsletters/export?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.LabID != 9 {
		t.Errorf("ListEligibleFamiliesForNewsletter LabID = %d, want 9", captured.LabID)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/csv")
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != "attachment; filename=newsletters.csv" {
		t.Errorf("Content-Disposition = %q, want %q", cd, "attachment; filename=newsletters.csv")
	}

	rows, err := csv.NewReader(rec.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV body: %v; body = %s", err, rec.Body)
	}
	want := [][]string{
		{"Jane", "Doe", "1 Main St", "Boulder", "CO", "80301"},
		{"John", "Smith", "2 Elm St", "Boulder", "CO", "80302"},
	}
	if len(rows) != len(want) {
		t.Fatalf("CSV rows = %+v, want %+v", rows, want)
	}
	for i := range want {
		for j := range want[i] {
			if rows[i][j] != want[i][j] {
				t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
			}
		}
	}
}

func TestHandleExportNewsletter_MissingDateRange(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/newsletters/export?start_date=2026-01-01", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleExportNewsletter_InvalidStartDate(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/newsletters/export?start_date=not-a-date&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleExportNewsletter_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		ListEligibleFamiliesForNewsletterFunc: func(ctx context.Context, arg db.ListEligibleFamiliesForNewsletterParams) ([]db.ListEligibleFamiliesForNewsletterRow, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/newsletters/export?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- mark-sent ---

func TestHandleMarkNewsletterSent_Success(t *testing.T) {
	var marked []db.MarkNewsletterSentParams
	q := &dbfake.Querier{
		GetNewsletterByIDFunc: func(ctx context.Context, id int64) (db.Newsletter, error) {
			return db.Newsletter{ID: id, LabID: 1}, nil
		},
		ListEligibleFamiliesForNewsletterFunc: func(ctx context.Context, arg db.ListEligibleFamiliesForNewsletterParams) ([]db.ListEligibleFamiliesForNewsletterRow, error) {
			if arg.NewsletterID == nil || *arg.NewsletterID != 3 {
				t.Errorf("NewsletterID filter = %v, want 3", arg.NewsletterID)
			}
			return []db.ListEligibleFamiliesForNewsletterRow{
				{FamilyID: 1, GuardianID: 10},
				{FamilyID: 2, GuardianID: 20},
			}, nil
		},
		MarkNewsletterSentFunc: func(ctx context.Context, arg db.MarkNewsletterSentParams) error {
			marked = append(marked, arg)
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/newsletters/3/mark-sent?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if len(marked) != 2 {
		t.Fatalf("MarkNewsletterSent calls = %d, want 2", len(marked))
	}
	if marked[0].NewsletterID != 3 || marked[0].GuardianID != 10 {
		t.Errorf("marked[0] = %+v", marked[0])
	}
	if marked[1].NewsletterID != 3 || marked[1].GuardianID != 20 {
		t.Errorf("marked[1] = %+v", marked[1])
	}
	got := decodeBody[map[string]int](t, rec)
	if got["marked_sent"] != 2 {
		t.Errorf("response = %+v, want marked_sent=2", got)
	}
}

func TestHandleMarkNewsletterSent_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetNewsletterByIDFunc: func(ctx context.Context, id int64) (db.Newsletter, error) {
			return db.Newsletter{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/newsletters/404/mark-sent?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleMarkNewsletterSent_MissingDateRange(t *testing.T) {
	q := &dbfake.Querier{
		GetNewsletterByIDFunc: func(ctx context.Context, id int64) (db.Newsletter, error) {
			return db.Newsletter{ID: id, LabID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/newsletters/3/mark-sent", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleMarkNewsletterSent_UnexpectedDBErrorMidLoop(t *testing.T) {
	q := &dbfake.Querier{
		GetNewsletterByIDFunc: func(ctx context.Context, id int64) (db.Newsletter, error) {
			return db.Newsletter{ID: id, LabID: 1}, nil
		},
		ListEligibleFamiliesForNewsletterFunc: func(ctx context.Context, arg db.ListEligibleFamiliesForNewsletterParams) ([]db.ListEligibleFamiliesForNewsletterRow, error) {
			return []db.ListEligibleFamiliesForNewsletterRow{
				{FamilyID: 1, GuardianID: 10},
				{FamilyID: 2, GuardianID: 20},
			}, nil
		},
		MarkNewsletterSentFunc: func(ctx context.Context, arg db.MarkNewsletterSentParams) error {
			return assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/newsletters/3/mark-sent?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
