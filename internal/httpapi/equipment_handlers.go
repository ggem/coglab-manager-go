package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionEquipmentCreated     = "equipment.created"
	ActionEquipmentUpdated     = "equipment.updated"
	ActionEquipmentDeactivated = "equipment.deactivated"
)

type equipmentRequest struct {
	Name     string `json:"name"`
	Quantity int16  `json:"quantity"`
}

type equipmentResponse struct {
	ID          int64     `json:"id"`
	LabID       int64     `json:"lab_id"`
	Name        string    `json:"name"`
	Quantity    int16     `json:"quantity"`
	Deactivated bool      `json:"deactivated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func equipmentToResponse(e db.Equipment) equipmentResponse {
	return equipmentResponse{
		ID:          e.ID,
		LabID:       e.LabID,
		Name:        e.Name,
		Quantity:    e.Quantity,
		Deactivated: e.DeactivatedAt.Valid,
		CreatedAt:   e.CreatedAt.Time,
		UpdatedAt:   e.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateEquipment(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req equipmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	equipment, err := s.queries.CreateEquipment(r.Context(), db.CreateEquipmentParams{
		LabID:    labID,
		Name:     req.Name,
		Quantity: req.Quantity,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &equipment.LabID,
		Action:      ActionEquipmentCreated,
		EntityType:  ptr("equipment"),
		EntityID:    &equipment.ID,
	})

	writeJSON(w, http.StatusCreated, equipmentToResponse(equipment))
}

func (s *Server) handleGetEquipment(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "equipmentID")
	if !ok {
		return
	}

	equipment, err := s.queries.GetEquipmentByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, equipmentToResponse(equipment))
}

func (s *Server) handleListEquipmentByLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	equipment, err := s.queries.ListEquipmentByLab(r.Context(), labID)
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

func (s *Server) handleUpdateEquipment(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "equipmentID")
	if !ok {
		return
	}

	var req equipmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	equipment, err := s.queries.UpdateEquipment(r.Context(), db.UpdateEquipmentParams{
		ID:       id,
		Name:     req.Name,
		Quantity: req.Quantity,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &equipment.LabID,
		Action:      ActionEquipmentUpdated,
		EntityType:  ptr("equipment"),
		EntityID:    &equipment.ID,
	})

	writeJSON(w, http.StatusOK, equipmentToResponse(equipment))
}

func (s *Server) handleDeactivateEquipment(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "equipmentID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateEquipment(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionEquipmentDeactivated,
		EntityType:  ptr("equipment"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}
