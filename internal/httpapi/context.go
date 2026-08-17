package httpapi

import (
	"context"
	"net/http"

	"github.com/ggem/coglab-manager-go/internal/db"
)

// contextKey is an unexported type so values this package stores in a
// request context can't collide with keys set by other packages, even if
// they also happen to use a string or int as their key type.
type contextKey int

const sessionContextKey contextKey = iota

func withSession(ctx context.Context, session db.Session) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

// sessionFromContext returns the session attached to ctx by requireAuth, if
// any. ok is false for requests that never passed through requireAuth.
func sessionFromContext(ctx context.Context) (session db.Session, ok bool) {
	session, ok = ctx.Value(sessionContextKey).(db.Session)
	return session, ok
}

// currentUserID returns the authenticated user's ID for use as an audit
// event's ActorUserID, or nil if ctx never passed through requireAuth.
func currentUserID(ctx context.Context) *int64 {
	session, ok := sessionFromContext(ctx)
	if !ok {
		return nil
	}
	return &session.UserID
}

// requireCurrentUserID returns the authenticated user's ID for use in a
// non-nullable field (e.g. a note's author). It writes a 500 and returns
// ok=false in the should-never-happen case where a route behind
// requireAuth has no session in its context.
func (s *Server) requireCurrentUserID(w http.ResponseWriter, r *http.Request) (userID int64, ok bool) {
	id := currentUserID(r.Context())
	if id == nil {
		s.logger.Error("no session in context for an authenticated route")
		writeError(w, http.StatusInternalServerError, "internal error")
		return 0, false
	}
	return *id, true
}
