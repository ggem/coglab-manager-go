package httpapi

import (
	"errors"
	"net/http"

	"github.com/ggem/coglab-manager-go/internal/auth"
)

// requireAuth resolves the session cookie on the request, rejects the
// request if it's missing, expired, or revoked, and otherwise attaches the
// session to the request context for downstream handlers.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := s.sessions.Validate(r.Context(), r)
		if err != nil {
			if errors.Is(err, auth.ErrSessionNotFound) || errors.Is(err, auth.ErrSessionExpired) || errors.Is(err, auth.ErrSessionRevoked) {
				writeError(w, http.StatusUnauthorized, "not authenticated")
				return
			}
			s.logger.Error("validate session", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if err := s.sessions.Touch(r.Context(), session.ID); err != nil {
			s.logger.Error("touch session", "error", err)
		}

		next.ServeHTTP(w, r.WithContext(withSession(r.Context(), session)))
	})
}
