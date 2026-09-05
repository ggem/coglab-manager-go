package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionExperimentTypeCreated     = "experiment_type.created"
	ActionExperimentTypeUpdated     = "experiment_type.updated"
	ActionExperimentTypeDeactivated = "experiment_type.deactivated"
)

type experimentTypeRequest struct {
	Name string `json:"name"`
}

type experimentTypeResponse struct {
	ID          int64     `json:"id"`
	LabID       int64     `json:"lab_id"`
	Name        string    `json:"name"`
	Deactivated bool      `json:"deactivated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func experimentTypeToResponse(t db.ExperimentType) experimentTypeResponse {
	return experimentTypeResponse{
		ID:          t.ID,
		LabID:       t.LabID,
		Name:        t.Name,
		Deactivated: t.DeactivatedAt.Valid,
		CreatedAt:   t.CreatedAt.Time,
		UpdatedAt:   t.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateExperimentType(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req experimentTypeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	experimentType, err := s.queries.CreateExperimentType(r.Context(), db.CreateExperimentTypeParams{
		LabID: labID,
		Name:  req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &experimentType.LabID,
		Action:      ActionExperimentTypeCreated,
		EntityType:  ptr("experiment_type"),
		EntityID:    &experimentType.ID,
	})

	writeJSON(w, http.StatusCreated, experimentTypeToResponse(experimentType))
}

func (s *Server) handleListExperimentTypesByLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	experimentTypes, err := s.queries.ListExperimentTypesByLab(r.Context(), labID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]experimentTypeResponse, len(experimentTypes))
	for i, t := range experimentTypes {
		resp[i] = experimentTypeToResponse(t)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateExperimentType(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "experimentTypeID")
	if !ok {
		return
	}

	var req experimentTypeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	experimentType, err := s.queries.UpdateExperimentType(r.Context(), db.UpdateExperimentTypeParams{
		ID:   id,
		Name: req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &experimentType.LabID,
		Action:      ActionExperimentTypeUpdated,
		EntityType:  ptr("experiment_type"),
		EntityID:    &experimentType.ID,
	})

	writeJSON(w, http.StatusOK, experimentTypeToResponse(experimentType))
}

func (s *Server) handleDeactivateExperimentType(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "experimentTypeID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateExperimentType(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionExperimentTypeDeactivated,
		EntityType:  ptr("experiment_type"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}
