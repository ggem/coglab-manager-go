package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionGrantCreated     = "grant.created"
	ActionGrantUpdated     = "grant.updated"
	ActionGrantDeactivated = "grant.deactivated"
)

type grantRequest struct {
	Name string `json:"name"`
}

type grantResponse struct {
	ID          int64     `json:"id"`
	LabID       int64     `json:"lab_id"`
	Name        string    `json:"name"`
	Deactivated bool      `json:"deactivated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func grantToResponse(g db.Grant) grantResponse {
	return grantResponse{
		ID:          g.ID,
		LabID:       g.LabID,
		Name:        g.Name,
		Deactivated: g.DeactivatedAt.Valid,
		CreatedAt:   g.CreatedAt.Time,
		UpdatedAt:   g.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateGrant(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req grantRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	grant, err := s.queries.CreateGrant(r.Context(), db.CreateGrantParams{
		LabID: labID,
		Name:  req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &grant.LabID,
		Action:      ActionGrantCreated,
		EntityType:  ptr("grant"),
		EntityID:    &grant.ID,
	})

	writeJSON(w, http.StatusCreated, grantToResponse(grant))
}

func (s *Server) handleGetGrant(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "grantID")
	if !ok {
		return
	}

	grant, err := s.queries.GetGrantByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, grantToResponse(grant))
}

func (s *Server) handleListGrantsByLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	grants, err := s.queries.ListGrantsByLab(r.Context(), labID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]grantResponse, len(grants))
	for i, g := range grants {
		resp[i] = grantToResponse(g)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateGrant(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "grantID")
	if !ok {
		return
	}

	var req grantRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	grant, err := s.queries.UpdateGrant(r.Context(), db.UpdateGrantParams{
		ID:   id,
		Name: req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &grant.LabID,
		Action:      ActionGrantUpdated,
		EntityType:  ptr("grant"),
		EntityID:    &grant.ID,
	})

	writeJSON(w, http.StatusOK, grantToResponse(grant))
}

func (s *Server) handleDeactivateGrant(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "grantID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateGrant(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionGrantDeactivated,
		EntityType:  ptr("grant"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}
