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
	ActionExperimentRoleSitterSet   = "experiment_role.sitter_set"
	ActionExperimentRoleGreeterSet  = "experiment_role.greeter_set"
)

type experimentRoleRequest struct {
	Name string `json:"name"`
}

type experimentRoleResponse struct {
	ID            int64     `json:"id"`
	LabID         int64     `json:"lab_id"`
	Name          string    `json:"name"`
	IsSitterRole  bool      `json:"is_sitter_role"`
	IsGreeterRole bool      `json:"is_greeter_role"`
	Deactivated   bool      `json:"deactivated"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func experimentRoleToResponse(role db.ExperimentRole) experimentRoleResponse {
	return experimentRoleResponse{
		ID:            role.ID,
		LabID:         role.LabID,
		Name:          role.Name,
		IsSitterRole:  role.IsSitterRole,
		IsGreeterRole: role.IsGreeterRole,
		Deactivated:   role.DeactivatedAt.Valid,
		CreatedAt:     role.CreatedAt.Time,
		UpdatedAt:     role.UpdatedAt.Time,
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

type setExperimentRoleSitterRequest struct {
	IsSitterRole bool `json:"is_sitter_role"`
}

// handleSetExperimentRoleSitter designates (or un-designates) a role as
// the lab's sitter role -- a dedicated action rather than folded into
// Update, since it's a distinct decision with its own constraint (at most
// one sitter role per lab, enforced by a partial unique index). Setting a
// second role true while one's already set is rejected as a conflict; the
// caller must unset the old one first.
func (s *Server) handleSetExperimentRoleSitter(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "roleID")
	if !ok {
		return
	}

	var req setExperimentRoleSitterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Setting true on a deactivated role is rejected by the query's WHERE
	// clause too (zero rows), but that alone would surface as a
	// misleading 404 (the role does exist) -- check explicitly here for a
	// clear 400 instead. Unsetting (false) skips this: always allowed,
	// even for a deactivated role, to clean up a stale flag.
	if req.IsSitterRole {
		existing, err := s.queries.GetExperimentRoleByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if existing.DeactivatedAt.Valid {
			writeError(w, http.StatusBadRequest, "cannot designate a deactivated role as the sitter role")
			return
		}
	}

	role, err := s.queries.SetExperimentRoleSitter(r.Context(), db.SetExperimentRoleSitterParams{
		ID:           id,
		IsSitterRole: req.IsSitterRole,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &role.LabID,
		Action:      ActionExperimentRoleSitterSet,
		EntityType:  ptr("experiment_role"),
		EntityID:    &role.ID,
		Metadata:    map[string]bool{"is_sitter_role": req.IsSitterRole},
	})

	writeJSON(w, http.StatusOK, experimentRoleToResponse(role))
}

type setExperimentRoleGreeterRequest struct {
	IsGreeterRole bool `json:"is_greeter_role"`
}

// handleSetExperimentRoleGreeter mirrors handleSetExperimentRoleSitter:
// designates (or un-designates) a role as the lab's dedicated-greeter
// role -- at most one per lab, enforced by a partial unique index.
// Setting a second role true while one's already set is rejected as a
// conflict; the caller must unset the old one first.
func (s *Server) handleSetExperimentRoleGreeter(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "roleID")
	if !ok {
		return
	}

	var req setExperimentRoleGreeterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Mirrors handleSetExperimentRoleSitter's deactivated-role guard: a
	// clear 400 rather than a misleading 404 from the query's WHERE
	// clause. Unsetting (false) always goes through.
	if req.IsGreeterRole {
		existing, err := s.queries.GetExperimentRoleByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if existing.DeactivatedAt.Valid {
			writeError(w, http.StatusBadRequest, "cannot designate a deactivated role as the greeter role")
			return
		}
	}

	role, err := s.queries.SetExperimentRoleGreeter(r.Context(), db.SetExperimentRoleGreeterParams{
		ID:            id,
		IsGreeterRole: req.IsGreeterRole,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &role.LabID,
		Action:      ActionExperimentRoleGreeterSet,
		EntityType:  ptr("experiment_role"),
		EntityID:    &role.ID,
		Metadata:    map[string]bool{"is_greeter_role": req.IsGreeterRole},
	})

	writeJSON(w, http.StatusOK, experimentRoleToResponse(role))
}
