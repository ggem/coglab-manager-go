package httpapi

import (
	"net/http"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionExperimentConditionAdded             = "experiment_condition.added"
	ActionExperimentConditionRemoved           = "experiment_condition.removed"
	ActionExperimentEquipmentAdded             = "experiment_equipment.added"
	ActionExperimentEquipmentRemoved           = "experiment_equipment.removed"
	ActionExperimentTrainingRequirementAdded   = "experiment_training_requirement.added"
	ActionExperimentTrainingRequirementRemoved = "experiment_training_requirement.removed"
)

type addConditionRequest struct {
	ConditionID int64 `json:"condition_id"`
}

func (s *Server) handleAddExperimentCondition(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	var req addConditionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.queries.AddExperimentCondition(r.Context(), db.AddExperimentConditionParams{
		ExperimentID: experimentID,
		ConditionID:  req.ConditionID,
	}); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionExperimentConditionAdded,
		EntityType:  ptr("experiment"),
		EntityID:    &experimentID,
		Metadata:    map[string]int64{"condition_id": req.ConditionID},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveExperimentCondition(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}
	conditionID, ok := idParam(w, r, "conditionID")
	if !ok {
		return
	}

	if err := s.queries.RemoveExperimentCondition(r.Context(), db.RemoveExperimentConditionParams{
		ExperimentID: experimentID,
		ConditionID:  conditionID,
	}); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionExperimentConditionRemoved,
		EntityType:  ptr("experiment"),
		EntityID:    &experimentID,
		Metadata:    map[string]int64{"condition_id": conditionID},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListExperimentConditions(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	conditions, err := s.queries.ListExperimentConditions(r.Context(), experimentID)
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

type addEquipmentRequest struct {
	EquipmentID int64 `json:"equipment_id"`
}

func (s *Server) handleAddExperimentEquipment(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	var req addEquipmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.queries.AddExperimentEquipment(r.Context(), db.AddExperimentEquipmentParams{
		ExperimentID: experimentID,
		EquipmentID:  req.EquipmentID,
	}); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionExperimentEquipmentAdded,
		EntityType:  ptr("experiment"),
		EntityID:    &experimentID,
		Metadata:    map[string]int64{"equipment_id": req.EquipmentID},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveExperimentEquipment(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}
	equipmentID, ok := idParam(w, r, "equipmentID")
	if !ok {
		return
	}

	if err := s.queries.RemoveExperimentEquipment(r.Context(), db.RemoveExperimentEquipmentParams{
		ExperimentID: experimentID,
		EquipmentID:  equipmentID,
	}); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionExperimentEquipmentRemoved,
		EntityType:  ptr("experiment"),
		EntityID:    &experimentID,
		Metadata:    map[string]int64{"equipment_id": equipmentID},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListExperimentEquipment(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	equipment, err := s.queries.ListExperimentEquipment(r.Context(), experimentID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]equipmentResponse, len(equipment))
	for i, e := range equipment {
		resp[i] = equipmentToResponse(e)
	}
	writeJSON(w, http.StatusOK, resp)
}

type addTrainingRequirementRequest struct {
	ExperimentRoleID int64 `json:"experiment_role_id"`
}

func (s *Server) handleAddExperimentTrainingRequirement(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	var req addTrainingRequirementRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.queries.AddExperimentTrainingRequirement(r.Context(), db.AddExperimentTrainingRequirementParams{
		ExperimentID:     experimentID,
		ExperimentRoleID: req.ExperimentRoleID,
	}); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionExperimentTrainingRequirementAdded,
		EntityType:  ptr("experiment"),
		EntityID:    &experimentID,
		Metadata:    map[string]int64{"experiment_role_id": req.ExperimentRoleID},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveExperimentTrainingRequirement(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}
	roleID, ok := idParam(w, r, "roleID")
	if !ok {
		return
	}

	if err := s.queries.RemoveExperimentTrainingRequirement(r.Context(), db.RemoveExperimentTrainingRequirementParams{
		ExperimentID:     experimentID,
		ExperimentRoleID: roleID,
	}); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionExperimentTrainingRequirementRemoved,
		EntityType:  ptr("experiment"),
		EntityID:    &experimentID,
		Metadata:    map[string]int64{"experiment_role_id": roleID},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListExperimentTrainingRequirements(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	roles, err := s.queries.ListExperimentTrainingRequirements(r.Context(), experimentID)
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
