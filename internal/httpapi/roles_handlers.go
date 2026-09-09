package httpapi

import "net/http"

type roleResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// handleListRoles lists the fixed, small set of permission-level roles
// (staff/coordinator/admin) a lab membership can be assigned -- not
// lab-scoped, so it needs no roleID/labID param, just authentication.
func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.queries.ListRoles(r.Context())
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]roleResponse, len(roles))
	for i, role := range roles {
		resp[i] = roleResponse{ID: role.ID, Name: role.Name, Description: role.Description}
	}
	writeJSON(w, http.StatusOK, resp)
}
