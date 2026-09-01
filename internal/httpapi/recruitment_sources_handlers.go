package httpapi

import (
	"net/http"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const ActionRecruitmentSourceCreated = "recruitment_source.created"

type recruitmentSourceRequest struct {
	Name string `json:"name"`
}

type recruitmentSourceResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func recruitmentSourceToResponse(rs db.RecruitmentSource) recruitmentSourceResponse {
	return recruitmentSourceResponse{ID: rs.ID, Name: rs.Name}
}

// handleListRecruitmentSources lists the active recruitment sources a
// child's create/edit form offers as a dropdown, alongside a free-text
// "other" field -- recruitment_sources isn't lab-scoped (children
// aren't either), so any authenticated user can list it.
func (s *Server) handleListRecruitmentSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.queries.ListActiveRecruitmentSources(r.Context())
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]recruitmentSourceResponse, len(sources))
	for i, rs := range sources {
		resp[i] = recruitmentSourceToResponse(rs)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateRecruitmentSource(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	var req recruitmentSourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	source, err := s.queries.CreateRecruitmentSource(r.Context(), req.Name)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		Action:      ActionRecruitmentSourceCreated,
		EntityType:  ptr("recruitment_source"),
		EntityID:    &source.ID,
	})

	writeJSON(w, http.StatusCreated, recruitmentSourceToResponse(source))
}
