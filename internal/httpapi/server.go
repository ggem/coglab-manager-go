// Package httpapi is the HTTP transport layer: routing, request/response
// encoding, and translating between HTTP and the domain packages
// (internal/auth, internal/audit, ...). It holds no business logic of its
// own -- a handler decodes a request, calls into a domain package, and
// encodes the result.
package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
	"github.com/ggem/coglab-manager-go/internal/db"
)

// Server holds the dependencies HTTP handlers need. authenticator is typed
// as an interface, since it's the one dependency here a caller might
// plausibly want to substitute (e.g. a test double, or a future SSO
// authenticator); sessions and audit are concrete types because
// *auth.SessionManager and *audit.Recorder are each the only real
// implementation and aren't swapped out. queries is used directly by
// handlers with no business logic beyond CRUD (families, guardians, ...);
// if that logic grows past what a handler should own, it gets its own
// domain package the way auth and audit already have.
type Server struct {
	authenticator auth.LocalAuthenticator
	sessions      *auth.SessionManager
	audit         *audit.Recorder
	queries       db.Querier
	logger        *slog.Logger
}

func NewServer(authenticator auth.LocalAuthenticator, sessions *auth.SessionManager, recorder *audit.Recorder, queries db.Querier, logger *slog.Logger) *Server {
	return &Server{
		authenticator: authenticator,
		sessions:      sessions,
		audit:         recorder,
		queries:       queries,
		logger:        logger,
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", s.handleHealthz)
	r.Post("/login", s.handleLogin)

	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)

		r.Post("/logout", s.handleLogout)

		r.Route("/families", func(r chi.Router) {
			r.Post("/", s.handleCreateFamily)
			r.Route("/{familyID}", func(r chi.Router) {
				r.Get("/", s.handleGetFamily)
				r.Put("/", s.handleUpdateFamily)
				r.Route("/guardians", func(r chi.Router) {
					r.Post("/", s.handleCreateGuardian)
					r.Get("/", s.handleListGuardiansByFamily)
				})
				r.Route("/children", func(r chi.Router) {
					r.Post("/", s.handleCreateChild)
					r.Get("/", s.handleListChildrenByFamily)
				})
			})
		})

		r.Route("/guardians/{guardianID}", func(r chi.Router) {
			r.Get("/", s.handleGetGuardian)
			r.Put("/", s.handleUpdateGuardian)
			r.Delete("/", s.handleDeleteGuardian)
		})

		r.Route("/children", func(r chi.Router) {
			r.Get("/search", s.handleSearchChildren)
			r.Route("/{childID}", func(r chi.Router) {
				r.Get("/", s.handleGetChild)
				r.Put("/", s.handleUpdateChild)
				r.Post("/deactivate", s.handleDeactivateChild)
				r.Route("/notes", func(r chi.Router) {
					r.Post("/", s.handleCreateChildNote)
					r.Get("/", s.handleListChildNotes)
				})
			})
		})
	})

	return r
}

// idParam parses the named chi URL parameter as an int64, writing a 400
// response and returning ok=false if it's missing or not a valid ID.
func idParam(w http.ResponseWriter, r *http.Request, name string) (id int64, ok bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return id, true
}

// recordAuditEvent writes an audit event and logs, rather than returns, any
// failure: a transient audit-write error shouldn't fail the request that
// triggered it, but it must never pass silently either.
func (s *Server) recordAuditEvent(r *http.Request, event audit.Event) {
	if err := s.audit.Record(r.Context(), event); err != nil {
		s.logger.Error("record audit event", "action", event.Action, "error", err)
	}
}
