package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleCreateChildNote_Success(t *testing.T) {
	var captured db.CreateNoteParams
	q := &dbfake.Querier{
		CreateNoteFunc: func(ctx context.Context, arg db.CreateNoteParams) (db.Note, error) {
			captured = arg
			return db.Note{ID: 1, AuthorUserID: arg.AuthorUserID, Body: arg.Body}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/children/9/notes/", cookie, noteRequest{Body: "called, left voicemail"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.EntityType != "child" || captured.EntityID != 9 {
		t.Errorf("CreateNote entity = %q/%d, want child/9", captured.EntityType, captured.EntityID)
	}
	if captured.AuthorUserID != 7 {
		t.Errorf("CreateNote AuthorUserID = %d, want 7 (from the session)", captured.AuthorUserID)
	}
}

func TestHandleListChildNotes_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListNotesByEntityFunc: func(ctx context.Context, arg db.ListNotesByEntityParams) ([]db.Note, error) {
			if arg.EntityType != "child" || arg.EntityID != 9 {
				t.Errorf("ListNotesByEntity called with %+v", arg)
			}
			return []db.Note{{ID: 1, Body: "note one"}, {ID: 2, Body: "note two"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/children/9/notes/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]noteResponse](t, rec)
	if len(got) != 2 {
		t.Errorf("got %d notes, want 2", len(got))
	}
}
