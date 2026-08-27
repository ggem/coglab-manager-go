package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionConditionCreated          = "condition.created"
	ActionConditionUpdated          = "condition.updated"
	ActionConditionDeactivated      = "condition.deactivated"
	ActionConditionValueCreated     = "condition_value.created"
	ActionConditionValueUpdated     = "condition_value.updated"
	ActionConditionValueDeactivated = "condition_value.deactivated"
)

type conditionRequest struct {
	Name string `json:"name"`
}

type conditionResponse struct {
	ID          int64     `json:"id"`
	LabID       int64     `json:"lab_id"`
	Name        string    `json:"name"`
	Deactivated bool      `json:"deactivated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func conditionToResponse(c db.Condition) conditionResponse {
	return conditionResponse{
		ID:          c.ID,
		LabID:       c.LabID,
		Name:        c.Name,
		Deactivated: c.DeactivatedAt.Valid,
		CreatedAt:   c.CreatedAt.Time,
		UpdatedAt:   c.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateCondition(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req conditionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	condition, err := s.queries.CreateCondition(r.Context(), db.CreateConditionParams{
		LabID: labID,
		Name:  req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &condition.LabID,
		Action:      ActionConditionCreated,
		EntityType:  ptr("condition"),
		EntityID:    &condition.ID,
	})

	writeJSON(w, http.StatusCreated, conditionToResponse(condition))
}

func (s *Server) handleGetCondition(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "conditionID")
	if !ok {
		return
	}

	condition, err := s.queries.GetConditionByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, conditionToResponse(condition))
}

func (s *Server) handleListConditionsByLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	conditions, err := s.queries.ListConditionsByLab(r.Context(), labID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]conditionResponse, len(conditions))
	for i, c := range conditions {
		resp[i] = conditionToResponse(c)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateCondition(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "conditionID")
	if !ok {
		return
	}

	var req conditionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	condition, err := s.queries.UpdateCondition(r.Context(), db.UpdateConditionParams{
		ID:   id,
		Name: req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &condition.LabID,
		Action:      ActionConditionUpdated,
		EntityType:  ptr("condition"),
		EntityID:    &condition.ID,
	})

	writeJSON(w, http.StatusOK, conditionToResponse(condition))
}

func (s *Server) handleDeactivateCondition(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "conditionID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateCondition(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionConditionDeactivated,
		EntityType:  ptr("condition"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}

type conditionValueRequest struct {
	Name string `json:"name"`
}

type conditionValueResponse struct {
	ID          int64     `json:"id"`
	ConditionID int64     `json:"condition_id"`
	Name        string    `json:"name"`
	Deactivated bool      `json:"deactivated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func conditionValueToResponse(v db.ConditionValue) conditionValueResponse {
	return conditionValueResponse{
		ID:          v.ID,
		ConditionID: v.ConditionID,
		Name:        v.Name,
		Deactivated: v.DeactivatedAt.Valid,
		CreatedAt:   v.CreatedAt.Time,
		UpdatedAt:   v.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateConditionValue(w http.ResponseWriter, r *http.Request) {
	conditionID, ok := idParam(w, r, "conditionID")
	if !ok {
		return
	}

	var req conditionValueRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	value, err := s.queries.CreateConditionValue(r.Context(), db.CreateConditionValueParams{
		ConditionID: conditionID,
		Name:        req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionConditionValueCreated,
		EntityType:  ptr("condition_value"),
		EntityID:    &value.ID,
	})

	writeJSON(w, http.StatusCreated, conditionValueToResponse(value))
}

func (s *Server) handleListConditionValuesByCondition(w http.ResponseWriter, r *http.Request) {
	conditionID, ok := idParam(w, r, "conditionID")
	if !ok {
		return
	}

	values, err := s.queries.ListConditionValuesByCondition(r.Context(), conditionID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]conditionValueResponse, len(values))
	for i, v := range values {
		resp[i] = conditionValueToResponse(v)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateConditionValue(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "valueID")
	if !ok {
		return
	}

	var req conditionValueRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	value, err := s.queries.UpdateConditionValue(r.Context(), db.UpdateConditionValueParams{
		ID:   id,
		Name: req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionConditionValueUpdated,
		EntityType:  ptr("condition_value"),
		EntityID:    &value.ID,
	})

	writeJSON(w, http.StatusOK, conditionValueToResponse(value))
}

func (s *Server) handleDeactivateConditionValue(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "valueID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateConditionValue(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionConditionValueDeactivated,
		EntityType:  ptr("condition_value"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}
