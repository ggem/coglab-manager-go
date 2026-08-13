package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	sessionCookieName = "coglab_session"
	sessionTTL        = 7 * 24 * time.Hour
	tokenByteLength   = 32
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
	ErrSessionRevoked  = errors.New("session revoked")
)

// SessionManager issues, validates, and revokes sessions backed by the
// sessions table, and manages the cookie that carries the session token.
//
// secureCookies is passed in explicitly by the caller (ultimately from a
// startup config value in cmd/api) rather than this package inspecting its
// environment itself -- e.g. by sniffing the hostname the way the legacy
// app decided "production" vs "development" by matching against a
// hardcoded list of known server hostnames. That worked until a server was
// renamed or replaced, at which point it silently fell through to the
// wrong behavior. Pushing the decision up to the caller keeps it explicit
// and in one place.
type SessionManager struct {
	queries       db.Querier
	secureCookies bool
}

func NewSessionManager(queries db.Querier, secureCookies bool) *SessionManager {
	return &SessionManager{queries: queries, secureCookies: secureCookies}
}

// Issue creates a new session for userID and sets the session cookie on w.
func (m *SessionManager) Issue(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int64) error {
	token, tokenHash, err := generateToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	var userAgent *string
	if ua := r.UserAgent(); ua != "" {
		userAgent = &ua
	}

	_, err = m.queries.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: tokenHash,
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(sessionTTL), Valid: true},
		IpAddress: clientIP(r),
		UserAgent: userAgent,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	m.setCookie(w, token, time.Now().Add(sessionTTL))
	return nil
}

// Validate reads the session cookie from r and returns the corresponding
// session if it exists, hasn't expired, and hasn't been revoked.
func (m *SessionManager) Validate(ctx context.Context, r *http.Request) (db.Session, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return db.Session{}, ErrSessionNotFound
	}

	session, err := m.queries.GetSessionByTokenHash(ctx, hashToken(cookie.Value))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Session{}, ErrSessionNotFound
		}
		return db.Session{}, fmt.Errorf("get session: %w", err)
	}

	if session.RevokedAt.Valid {
		return db.Session{}, ErrSessionRevoked
	}
	if session.ExpiresAt.Valid && session.ExpiresAt.Time.Before(time.Now()) {
		return db.Session{}, ErrSessionExpired
	}

	return session, nil
}

// Touch updates the session's last_seen_at to now.
func (m *SessionManager) Touch(ctx context.Context, sessionID int64) error {
	if err := m.queries.TouchSessionLastSeen(ctx, sessionID); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// Revoke invalidates the session named by the cookie on r, if any, and
// clears the cookie on w.
func (m *SessionManager) Revoke(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := m.queries.RevokeSession(ctx, hashToken(cookie.Value)); err != nil {
			return fmt.Errorf("revoke session: %w", err)
		}
	}

	m.clearCookie(w)
	return nil
}

func (m *SessionManager) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secureCookies,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
}

// clearCookie tells the browser to delete the session cookie immediately.
// MaxAge < 0 is the documented net/http idiom for "delete this cookie now"
// and takes precedence over Expires in every modern browser; Expires is
// also set to the epoch for older clients that only understand it.
func (m *SessionManager) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secureCookies,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

// generateToken returns a fresh opaque session token along with the SHA-256
// hash that gets stored in the database. Only the hash is ever persisted;
// the raw token exists only in the cookie, so a database leak alone can't
// be used to impersonate a session.
func generateToken() (token string, hash []byte, err error) {
	raw := make([]byte, tokenByteLength)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func clientIP(r *http.Request) *netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return nil
	}
	return &addr
}
