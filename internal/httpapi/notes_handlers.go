package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const ActionNoteCreated = "note.created"

type noteRequest struct {
	Body string `json:"body"`
}

type noteResponse struct {
	ID           int64     `json:"id"`
	AuthorUserID int64     `json:"author_user_id"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
}

func noteToResponse(n db.Note) noteResponse {
	return noteResponse{
		ID:           n.ID,
		AuthorUserID: n.AuthorUserID,
		Body:         n.Body,
		CreatedAt:    n.CreatedAt.Time,
	}
}

func (s *Server) handleCreateChildNote(w http.ResponseWriter, r *http.Request) {
	childID, ok := idParam(w, r, "childID")
	if !ok {
		return
	}
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	var req noteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	note, err := s.queries.CreateNote(r.Context(), db.CreateNoteParams{
		EntityType:   "child",
		EntityID:     childID,
		AuthorUserID: userID,
		Body:         req.Body,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		Action:      ActionNoteCreated,
		EntityType:  ptr("child"),
		EntityID:    &childID,
	})

	writeJSON(w, http.StatusCreated, noteToResponse(note))
}

func (s *Server) handleListChildNotes(w http.ResponseWriter, r *http.Request) {
	childID, ok := idParam(w, r, "childID")
	if !ok {
		return
	}

	notes, err := s.queries.ListNotesByEntity(r.Context(), db.ListNotesByEntityParams{
		EntityType: "child",
		EntityID:   childID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]noteResponse, len(notes))
	for i, n := range notes {
		resp[i] = noteToResponse(n)
	}
	writeJSON(w, http.StatusOK, resp)
}
