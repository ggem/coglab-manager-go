package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionExperimentRoleCreated     = "experiment_role.created"
	ActionExperimentRoleUpdated     = "experiment_role.updated"
	ActionExperimentRoleDeactivated = "experiment_role.deactivated"
)

type experimentRoleRequest struct {
	Name string `json:"name"`
}

type experimentRoleResponse struct {
	ID          int64     `json:"id"`
	LabID       int64     `json:"lab_id"`
	Name        string    `json:"name"`
	Deactivated bool      `json:"deactivated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func experimentRoleToResponse(role db.ExperimentRole) experimentRoleResponse {
	return experimentRoleResponse{
		ID:          role.ID,
		LabID:       role.LabID,
		Name:        role.Name,
		Deactivated: role.DeactivatedAt.Valid,
		CreatedAt:   role.CreatedAt.Time,
		UpdatedAt:   role.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateExperimentRole(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req experimentRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	role, err := s.queries.CreateExperimentRole(r.Context(), db.CreateExperimentRoleParams{
		LabID: labID,
		Name:  req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &role.LabID,
		Action:      ActionExperimentRoleCreated,
		EntityType:  ptr("experiment_role"),
		EntityID:    &role.ID,
	})

	writeJSON(w, http.StatusCreated, experimentRoleToResponse(role))
}

func (s *Server) handleGetExperimentRole(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "roleID")
	if !ok {
		return
	}

	role, err := s.queries.GetExperimentRoleByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, experimentRoleToResponse(role))
}

func (s *Server) handleListExperimentRolesByLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	roles, err := s.queries.ListExperimentRolesByLab(r.Context(), labID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]experimentRoleResponse, len(roles))
	for i, role := range roles {
		resp[i] = experimentRoleToResponse(role)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateExperimentRole(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "roleID")
	if !ok {
		return
	}

	var req experimentRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	role, err := s.queries.UpdateExperimentRole(r.Context(), db.UpdateExperimentRoleParams{
		ID:   id,
		Name: req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &role.LabID,
		Action:      ActionExperimentRoleUpdated,
		EntityType:  ptr("experiment_role"),
		EntityID:    &role.ID,
	})

	writeJSON(w, http.StatusOK, experimentRoleToResponse(role))
}

func (s *Server) handleDeactivateExperimentRole(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "roleID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateExperimentRole(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionExperimentRoleDeactivated,
		EntityType:  ptr("experiment_role"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}
