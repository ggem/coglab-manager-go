package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionProtocolCreated     = "protocol.created"
	ActionProtocolUpdated     = "protocol.updated"
	ActionProtocolDeactivated = "protocol.deactivated"
)

type protocolRequest struct {
	Name string `json:"name"`
}

type protocolResponse struct {
	ID          int64     `json:"id"`
	LabID       int64     `json:"lab_id"`
	Name        string    `json:"name"`
	Deactivated bool      `json:"deactivated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func protocolToResponse(p db.Protocol) protocolResponse {
	return protocolResponse{
		ID:          p.ID,
		LabID:       p.LabID,
		Name:        p.Name,
		Deactivated: p.DeactivatedAt.Valid,
		CreatedAt:   p.CreatedAt.Time,
		UpdatedAt:   p.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateProtocol(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req protocolRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	protocol, err := s.queries.CreateProtocol(r.Context(), db.CreateProtocolParams{
		LabID: labID,
		Name:  req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &protocol.LabID,
		Action:      ActionProtocolCreated,
		EntityType:  ptr("protocol"),
		EntityID:    &protocol.ID,
	})

	writeJSON(w, http.StatusCreated, protocolToResponse(protocol))
}

func (s *Server) handleGetProtocol(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "protocolID")
	if !ok {
		return
	}

	protocol, err := s.queries.GetProtocolByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, protocolToResponse(protocol))
}

func (s *Server) handleListProtocolsByLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	protocols, err := s.queries.ListProtocolsByLab(r.Context(), labID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]protocolResponse, len(protocols))
	for i, p := range protocols {
		resp[i] = protocolToResponse(p)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateProtocol(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "protocolID")
	if !ok {
		return
	}

	var req protocolRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	protocol, err := s.queries.UpdateProtocol(r.Context(), db.UpdateProtocolParams{
		ID:   id,
		Name: req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &protocol.LabID,
		Action:      ActionProtocolUpdated,
		EntityType:  ptr("protocol"),
		EntityID:    &protocol.ID,
	})

	writeJSON(w, http.StatusOK, protocolToResponse(protocol))
}

func (s *Server) handleDeactivateProtocol(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "protocolID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateProtocol(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionProtocolDeactivated,
		EntityType:  ptr("protocol"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}
