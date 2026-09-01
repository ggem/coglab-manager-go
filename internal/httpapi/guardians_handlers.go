package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionGuardianCreated     = "guardian.created"
	ActionGuardianUpdated     = "guardian.updated"
	ActionGuardianDeactivated = "guardian.deactivated"
)

type guardianRequest struct {
	FirstName   string  `json:"first_name"`
	LastName    string  `json:"last_name"`
	Education   string  `json:"education"`
	Occupation  string  `json:"occupation"`
	PhoneNumber string  `json:"phone_number"`
	PhoneType   *string `json:"phone_type"`
	Email       string  `json:"email"`
}

type guardianResponse struct {
	ID          int64     `json:"id"`
	FamilyID    int64     `json:"family_id"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	Education   string    `json:"education"`
	Occupation  string    `json:"occupation"`
	PhoneNumber string    `json:"phone_number"`
	PhoneType   *string   `json:"phone_type"`
	Email       string    `json:"email"`
	Deactivated bool      `json:"deactivated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func guardianToResponse(g db.Guardian) guardianResponse {
	return guardianResponse{
		ID:          g.ID,
		FamilyID:    g.FamilyID,
		FirstName:   g.FirstName,
		LastName:    g.LastName,
		Education:   g.Education,
		Occupation:  g.Occupation,
		PhoneNumber: g.PhoneNumber,
		PhoneType:   g.PhoneType,
		Email:       g.Email,
		Deactivated: g.DeactivatedAt.Valid,
		CreatedAt:   g.CreatedAt.Time,
		UpdatedAt:   g.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateGuardian(w http.ResponseWriter, r *http.Request) {
	familyID, ok := idParam(w, r, "familyID")
	if !ok {
		return
	}

	var req guardianRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	guardian, err := s.queries.CreateGuardian(r.Context(), db.CreateGuardianParams{
		FamilyID:    familyID,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Education:   req.Education,
		Occupation:  req.Occupation,
		PhoneNumber: req.PhoneNumber,
		PhoneType:   req.PhoneType,
		Email:       req.Email,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionGuardianCreated,
		EntityType:  ptr("guardian"),
		EntityID:    &guardian.ID,
	})

	writeJSON(w, http.StatusCreated, guardianToResponse(guardian))
}

func (s *Server) handleListGuardiansByFamily(w http.ResponseWriter, r *http.Request) {
	familyID, ok := idParam(w, r, "familyID")
	if !ok {
		return
	}

	guardians, err := s.queries.ListGuardiansByFamily(r.Context(), familyID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]guardianResponse, len(guardians))
	for i, g := range guardians {
		resp[i] = guardianToResponse(g)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetGuardian(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "guardianID")
	if !ok {
		return
	}

	guardian, err := s.queries.GetGuardianByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, guardianToResponse(guardian))
}

func (s *Server) handleUpdateGuardian(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "guardianID")
	if !ok {
		return
	}

	var req guardianRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	guardian, err := s.queries.UpdateGuardian(r.Context(), db.UpdateGuardianParams{
		ID:          id,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Education:   req.Education,
		Occupation:  req.Occupation,
		PhoneNumber: req.PhoneNumber,
		PhoneType:   req.PhoneType,
		Email:       req.Email,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionGuardianUpdated,
		EntityType:  ptr("guardian"),
		EntityID:    &guardian.ID,
	})

	writeJSON(w, http.StatusOK, guardianToResponse(guardian))
}

func (s *Server) handleDeactivateGuardian(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "guardianID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateGuardian(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionGuardianDeactivated,
		EntityType:  ptr("guardian"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}
