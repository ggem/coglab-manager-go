package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionZipCodeCreated     = "zip_code.created"
	ActionZipCodeUpdated     = "zip_code.updated"
	ActionZipCodeDeactivated = "zip_code.deactivated"
)

type zipCodeRequest struct {
	ZipCode  string `json:"zip_code"`
	Priority string `json:"priority"`
}

type zipCodeResponse struct {
	ID          int64     `json:"id"`
	LabID       int64     `json:"lab_id"`
	ZipCode     string    `json:"zip_code"`
	Priority    string    `json:"priority"`
	Deactivated bool      `json:"deactivated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func zipCodeToResponse(z db.Zipcode) zipCodeResponse {
	return zipCodeResponse{
		ID:          z.ID,
		LabID:       z.LabID,
		ZipCode:     z.ZipCode,
		Priority:    z.Priority,
		Deactivated: z.DeactivatedAt.Valid,
		CreatedAt:   z.CreatedAt.Time,
		UpdatedAt:   z.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateZipCode(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req zipCodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	zipCode, err := s.queries.CreateZipCode(r.Context(), db.CreateZipCodeParams{
		LabID:    labID,
		ZipCode:  req.ZipCode,
		Priority: req.Priority,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &zipCode.LabID,
		Action:      ActionZipCodeCreated,
		EntityType:  ptr("zip_code"),
		EntityID:    &zipCode.ID,
	})

	writeJSON(w, http.StatusCreated, zipCodeToResponse(zipCode))
}

func (s *Server) handleGetZipCode(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "zipCodeID")
	if !ok {
		return
	}

	zipCode, err := s.queries.GetZipCodeByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, zipCodeToResponse(zipCode))
}

func (s *Server) handleListZipCodesByLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	zipCodes, err := s.queries.ListZipCodesByLab(r.Context(), labID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]zipCodeResponse, len(zipCodes))
	for i, z := range zipCodes {
		resp[i] = zipCodeToResponse(z)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateZipCode(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "zipCodeID")
	if !ok {
		return
	}

	var req zipCodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	zipCode, err := s.queries.UpdateZipCode(r.Context(), db.UpdateZipCodeParams{
		ID:       id,
		ZipCode:  req.ZipCode,
		Priority: req.Priority,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &zipCode.LabID,
		Action:      ActionZipCodeUpdated,
		EntityType:  ptr("zip_code"),
		EntityID:    &zipCode.ID,
	})

	writeJSON(w, http.StatusOK, zipCodeToResponse(zipCode))
}

func (s *Server) handleDeactivateZipCode(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "zipCodeID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateZipCode(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionZipCodeDeactivated,
		EntityType:  ptr("zip_code"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}
