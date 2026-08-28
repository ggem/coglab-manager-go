package httpapi

import (
	"net/http"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionLabMemberTrainingAdded   = "lab_member_training.added"
	ActionLabMemberTrainingRemoved = "lab_member_training.removed"
)

type addLabMemberTrainingRequest struct {
	UserID int64 `json:"user_id"`
}

// trainedMemberResponse is deliberately smaller than a full user
// response: this endpoint exists to answer "who's qualified for this
// role," not to expose full account details to whoever's assembling a
// candidate list.
type trainedMemberResponse struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (s *Server) handleAddLabMemberTraining(w http.ResponseWriter, r *http.Request) {
	roleID, ok := idParam(w, r, "roleID")
	if !ok {
		return
	}

	var req addLabMemberTrainingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.queries.AddLabMemberTraining(r.Context(), db.AddLabMemberTrainingParams{
		UserID:           req.UserID,
		ExperimentRoleID: roleID,
	}); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionLabMemberTrainingAdded,
		EntityType:  ptr("experiment_role"),
		EntityID:    &roleID,
		Metadata:    map[string]int64{"user_id": req.UserID},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveLabMemberTraining(w http.ResponseWriter, r *http.Request) {
	roleID, ok := idParam(w, r, "roleID")
	if !ok {
		return
	}
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}

	if err := s.queries.RemoveLabMemberTraining(r.Context(), db.RemoveLabMemberTrainingParams{
		UserID:           userID,
		ExperimentRoleID: roleID,
	}); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionLabMemberTrainingRemoved,
		EntityType:  ptr("experiment_role"),
		EntityID:    &roleID,
		Metadata:    map[string]int64{"user_id": userID},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListLabMemberTrainingsForRole(w http.ResponseWriter, r *http.Request) {
	roleID, ok := idParam(w, r, "roleID")
	if !ok {
		return
	}

	users, err := s.queries.ListLabMemberTrainingsForRole(r.Context(), roleID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]trainedMemberResponse, len(users))
	for i, u := range users {
		resp[i] = trainedMemberResponse{ID: u.ID, FirstName: u.FirstName, LastName: u.LastName}
	}
	writeJSON(w, http.StatusOK, resp)
}
