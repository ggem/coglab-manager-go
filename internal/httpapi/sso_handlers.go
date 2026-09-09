package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
)

// oidcStateCookie and oidcNonceCookie carry the per-attempt state/nonce
// between handleSSOLogin and handleSSOCallback. Short-lived (10 minutes --
// comfortably more than anyone takes to log in at an IdP, not so long
// that an abandoned attempt's cookie lingers) rather than sessionTTL-lived
// like the real session cookie, since these exist only to survive one
// redirect round trip.
const (
	oidcStateCookie   = "coglab_oidc_state"
	oidcNonceCookie   = "coglab_oidc_nonce"
	oidcCookieMaxAge  = 10 * time.Minute
	oidcRandomByteLen = 32
)

type ssoConfigResponse struct {
	Enabled bool `json:"enabled"`
}

// handleSSOConfig lets the frontend show (or hide) the "Sign in with SSO"
// option without guessing from a 404 -- unauthenticated, since it runs
// before anyone has a session.
func (s *Server) handleSSOConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, ssoConfigResponse{Enabled: s.oidc != nil})
}

// handleSSOLogin starts the OIDC flow: generates this attempt's state and
// nonce, stashes them in short-lived cookies so handleSSOCallback can
// verify them, and redirects the browser to the IdP.
func (s *Server) handleSSOLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomOIDCToken()
	if err != nil {
		s.logger.Error("generate oidc state", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	nonce, err := randomOIDCToken()
	if err != nil {
		s.logger.Error("generate oidc nonce", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	s.setOIDCCookie(w, oidcStateCookie, state)
	s.setOIDCCookie(w, oidcNonceCookie, nonce)

	http.Redirect(w, r, s.oidc.AuthCodeURL(state, nonce), http.StatusFound)
}

// handleSSOCallback is where the IdP redirects back to after the person
// authenticates there. State/nonce are read back out of the cookies set
// by handleSSOLogin, not trusted from the query string alone -- that's
// the actual CSRF protection: an attacker can craft a callback URL with
// any state value they like, but can't make the victim's browser present
// a cookie the attacker doesn't control.
func (s *Server) handleSSOCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, stateErr := r.Cookie(oidcStateCookie)
	nonceCookie, nonceErr := r.Cookie(oidcNonceCookie)
	s.clearOIDCCookie(w, oidcStateCookie)
	s.clearOIDCCookie(w, oidcNonceCookie)

	if stateErr != nil || nonceErr != nil || r.URL.Query().Get("state") != stateCookie.Value {
		s.recordAuditEvent(r, audit.Event{
			Action:   auth.ActionLoginFailed,
			Metadata: map[string]string{"reason": "sso_state_mismatch"},
		})
		http.Redirect(w, r, "/?sso_error=1", http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		s.recordAuditEvent(r, audit.Event{
			Action:   auth.ActionLoginFailed,
			Metadata: map[string]string{"reason": "sso_no_code"},
		})
		http.Redirect(w, r, "/?sso_error=1", http.StatusFound)
		return
	}

	identity, err := s.oidc.Exchange(r.Context(), code, nonceCookie.Value)
	if err != nil {
		s.logger.Error("sso exchange", "error", err)
		s.recordAuditEvent(r, audit.Event{
			Action:   auth.ActionLoginFailed,
			Metadata: map[string]string{"reason": "sso_exchange_failed"},
		})
		http.Redirect(w, r, "/?sso_error=1", http.StatusFound)
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
		Metadata:    map[string]string{"via": "sso"},
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func randomOIDCToken() (string, error) {
	raw := make([]byte, oidcRandomByteLen)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Server) setOIDCCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/auth/sso",
		HttpOnly: true,
		Secure:   s.sessions.SecureCookies(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(oidcCookieMaxAge),
	})
}

func (s *Server) clearOIDCCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/auth/sso",
		HttpOnly: true,
		Secure:   s.sessions.SecureCookies(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}
