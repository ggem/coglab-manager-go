package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/db"
)

type labResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	ShortName string    `json:"short_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func labToResponse(l db.Lab) labResponse {
	return labResponse{
		ID:        l.ID,
		Name:      l.Name,
		ShortName: l.ShortName,
		CreatedAt: l.CreatedAt.Time,
		UpdatedAt: l.UpdatedAt.Time,
	}
}

// handleListMyLabs lists the labs the current user belongs to -- the
// only way a client can discover a lab ID to use with the rest of the
// lab-scoped API, which otherwise always takes one as a given.
func (s *Server) handleListMyLabs(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	labs, err := s.queries.ListLabsForUser(r.Context(), userID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]labResponse, len(labs))
	for i, l := range labs {
		resp[i] = labToResponse(l)
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleListLabs is the platform-admin counterpart to handleListMyLabs:
// every lab in the system, not just ones the caller belongs to -- needed
// for the "assign a lab" picker on the Users page, since a platform
// admin creating an account isn't necessarily a member of the lab
// they're assigning it to.
func (s *Server) handleListLabs(w http.ResponseWriter, r *http.Request) {
	labs, err := s.queries.ListLabs(r.Context())
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]labResponse, len(labs))
	for i, l := range labs {
		resp[i] = labToResponse(l)
	}
	writeJSON(w, http.StatusOK, resp)
}
