package httpapi

import "net/http"

// handleListLabMembers lists who belongs to a lab -- the candidate
// pool a picker (e.g. principal investigators) draws from. Reuses
// trainedMemberResponse's deliberately minimal shape (id/first/last
// name only), same rationale as lab_member_trainings_handlers.go.
func (s *Server) handleListLabMembers(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	users, err := s.queries.ListLabMembers(r.Context(), labID)
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
