// Package httpapi is the HTTP transport layer: routing, request/response
// encoding, and translating between HTTP and the domain packages
// (internal/auth, internal/audit, ...). It holds no business logic of its
// own -- a handler decodes a request, calls into a domain package, and
// encodes the result.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
)

// Server holds the dependencies HTTP handlers need. authenticator is typed
// as an interface, since it's the one dependency here a caller might
// plausibly want to substitute (e.g. a test double, or a future SSO
// authenticator); sessions and audit are concrete types because
// *auth.SessionManager and *audit.Recorder are each the only real
// implementation and aren't swapped out.
type Server struct {
	authenticator auth.LocalAuthenticator
	sessions      *auth.SessionManager
	audit         *audit.Recorder
	logger        *slog.Logger
}

func NewServer(authenticator auth.LocalAuthenticator, sessions *auth.SessionManager, recorder *audit.Recorder, logger *slog.Logger) *Server {
	return &Server{
		authenticator: authenticator,
		sessions:      sessions,
		audit:         recorder,
		logger:        logger,
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", s.handleHealthz)
	r.Post("/login", s.handleLogin)
	r.With(s.requireAuth).Post("/logout", s.handleLogout)

	return r
}

// recordAuditEvent writes an audit event and logs, rather than returns, any
// failure: a transient audit-write error shouldn't fail the request that
// triggered it, but it must never pass silently either.
func (s *Server) recordAuditEvent(r *http.Request, event audit.Event) {
	if err := s.audit.Record(r.Context(), event); err != nil {
		s.logger.Error("record audit event", "action", event.Action, "error", err)
	}
}
