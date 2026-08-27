package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

// newAuthenticatedTestServer wires up q with a valid session for userID
// (unless the test already set its own session stubs) and returns a
// server plus the cookie a request needs to authenticate as that user.
func newAuthenticatedTestServer(q *dbfake.Querier, userID int64) (*Server, *http.Cookie) {
	if q.GetSessionByTokenHashFunc == nil {
		q.GetSessionByTokenHashFunc = func(ctx context.Context, tokenHash []byte) (db.Session, error) {
			return db.Session{ID: 1, UserID: userID}, nil
		}
	}
	if q.TouchSessionLastSeenFunc == nil {
		q.TouchSessionLastSeenFunc = func(ctx context.Context, id int64) error { return nil }
	}
	if q.GetLabMembershipFunc == nil {
		q.GetLabMembershipFunc = func(ctx context.Context, arg db.GetLabMembershipParams) (db.LabMembership, error) {
			return db.LabMembership{UserID: arg.UserID, LabID: arg.LabID}, nil
		}
	}
	s := newTestServer(q)
	cookie := &http.Cookie{Name: sessionCookieNameForTest, Value: "test-token"}
	return s, cookie
}

// doRequest performs an HTTP request against s.Routes(), optionally with a
// session cookie and a JSON-encoded body (nil for none).
func doRequest(t *testing.T, s *Server, method, path string, cookie *http.Cookie, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("unmarshal response body: %v; body = %s", err, rec.Body)
	}
	return v
}
