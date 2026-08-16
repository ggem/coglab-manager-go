package httpapi

import (
	"errors"
	"net/http"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type loginResponse struct {
	User userResponse `json:"user"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	identity, err := s.authenticator.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		s.handleLoginFailure(w, r, req.Email, err)
		return
	}

	if err := s.sessions.Issue(r.Context(), w, r, identity.UserID); err != nil {
		s.logger.Error("issue session", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	userID := identity.UserID
	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		Action:      auth.ActionLoginSucceeded,
	})

	writeJSON(w, http.StatusOK, loginResponse{User: userResponse{
		ID:        identity.UserID,
		Email:     identity.Email,
		FirstName: identity.FirstName,
		LastName:  identity.LastName,
	}})
}

// handleLoginFailure records why a login attempt failed and responds with a
// generic 401. The HTTP response deliberately doesn't distinguish "unknown
// email" from "wrong password" from "deactivated account" -- that
// distinction is exactly what would let an attacker enumerate valid
// accounts. The audit trail is not exposed to the client making the
// request, so it keeps the real reason for admins investigating later.
func (s *Server) handleLoginFailure(w http.ResponseWriter, r *http.Request, email string, err error) {
	var reason string
	switch {
	case errors.Is(err, auth.ErrAccountDeactivated):
		reason = "account_deactivated"
	case errors.Is(err, auth.ErrInvalidCredentials):
		reason = "invalid_credentials"
	default:
		s.logger.Error("authenticate", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	s.recordAuditEvent(r, audit.Event{
		Action:   auth.ActionLoginFailed,
		Metadata: map[string]string{"email": email, "reason": reason},
	})
	writeError(w, http.StatusUnauthorized, "invalid credentials")
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionFromContext(r.Context())

	if err := s.sessions.Revoke(r.Context(), w, r); err != nil {
		s.logger.Error("revoke session", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	userID := session.UserID
	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		Action:      auth.ActionLogout,
	})

	w.WriteHeader(http.StatusNoContent)
}
