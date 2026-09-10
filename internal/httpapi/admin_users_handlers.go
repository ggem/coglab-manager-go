package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/mail"
)

// adminUserResponse is deliberately different from userResponse (the
// caller's own /me profile): has_password answers "has this account
// been activated yet" for the platform-admin listing, a question that
// makes no sense to ask about yourself.
type adminUserResponse struct {
	ID              int64  `json:"id"`
	Email           string `json:"email"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	IsPlatformAdmin bool   `json:"is_platform_admin"`
	HasPassword     bool   `json:"has_password"`
	Deactivated     bool   `json:"deactivated"`
}

func adminUserToResponse(u db.User) adminUserResponse {
	return adminUserResponse{
		ID:              u.ID,
		Email:           u.Email,
		FirstName:       u.FirstName,
		LastName:        u.LastName,
		IsPlatformAdmin: u.IsPlatformAdmin,
		HasPassword:     u.PasswordHash != nil,
		Deactivated:     u.DeactivatedAt.Valid,
	}
}

// handleListUsers is the platform-admin Users page's roster -- every
// account in the system, not scoped to any one lab (there's no
// lab-scoped equivalent: handleListLabMembers/handleListLabMemberships
// already cover "who's in this lab").
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.queries.ListUsers(r.Context())
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]adminUserResponse, len(users))
	for i, u := range users {
		resp[i] = adminUserToResponse(u)
	}
	writeJSON(w, http.StatusOK, resp)
}

type createUserRequest struct {
	Email           string `json:"email"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	IsPlatformAdmin bool   `json:"is_platform_admin"`
}

// createUserResponse reports whether the invite email actually went out,
// rather than a bare 201 that can't distinguish "created and notified"
// from "created, but the person has no way to find out" -- see
// sendInviteEmail's doc comment for why that distinction matters and
// InviteEmailSent is the caller's only way to react to it.
type createUserResponse struct {
	adminUserResponse
	InviteEmailSent bool `json:"invite_email_sent"`
}

// handleCreateUser is the platform-admin counterpart to
// handleCreateLabMembershipForNewUser: creates a standalone account (not
// tied to any lab yet) and, unlike that lab-scoped path, lets the caller
// grant platform-admin on the new account -- safe here because only an
// existing platform admin can reach this route at all, unlike a lab
// admin's more limited authority.
//
// CreateUser and CreateInviteToken run in the same transaction: if token
// creation fails (a transient DB error, say), the user row is rolled
// back too, so the caller can just retry the identical request rather
// than being stuck on a unique-email conflict with an account that has
// no working invite and no built-in way to get one. Only the network
// call to actually send the email happens after commit -- see
// sendInviteEmail.
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.FirstName == "" || req.LastName == "" {
		writeError(w, http.StatusBadRequest, "email, first_name, and last_name are required")
		return
	}

	var user db.User
	var token string
	txErr := s.withTx(r.Context(), func(q db.Querier) error {
		var err error
		user, err = q.CreateUser(r.Context(), db.CreateUserParams{
			Email:           req.Email,
			FirstName:       req.FirstName,
			LastName:        req.LastName,
			PasswordHash:    nil,
			IsPlatformAdmin: req.IsPlatformAdmin,
		})
		if err != nil {
			return err
		}
		token, err = auth.CreateInviteToken(r.Context(), q, user.ID)
		return err
	})
	if txErr != nil {
		s.writeDBError(w, txErr)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      auth.ActionUserCreated,
		EntityType:  ptr("user"),
		EntityID:    &user.ID,
		Metadata:    map[string]any{"email": user.Email, "is_platform_admin": user.IsPlatformAdmin},
	})

	sent := s.sendInviteEmail(r.Context(), user, token)

	writeJSON(w, http.StatusCreated, createUserResponse{adminUserToResponse(user), sent})
}

// resendInviteResponse is handleResendInvite's response -- just the one
// fact its caller needs, not a full adminUserResponse (nothing else
// about the account changed).
type resendInviteResponse struct {
	InviteEmailSent bool `json:"invite_email_sent"`
}

// handleResendInvite is the recovery path for an account whose original
// invite email never arrived (an SMTP outage, a typo since corrected,
// ...): generates a fresh token and re-sends. Without this, a failed
// send after handleCreateUser's transaction has already committed left
// a genuinely-created, unrecoverable account -- the person has no
// working link, and the admin can't just create the account again
// (unique email). Rejects an already-activated account (400): this is
// for finishing first-time setup, not a general password-reset tool.
func (s *Server) handleResendInvite(w http.ResponseWriter, r *http.Request) {
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}

	user, err := s.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	if user.PasswordHash != nil {
		writeError(w, http.StatusBadRequest, "this account has already been activated")
		return
	}

	token, err := auth.CreateInviteToken(r.Context(), s.queries, user.ID)
	if err != nil {
		s.logger.Error("create invite token", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	sent := s.sendInviteEmail(r.Context(), user, token)

	writeJSON(w, http.StatusOK, resendInviteResponse{InviteEmailSent: sent})
}

// handleDeactivateUser blocks an account from logging in ever again
// (matching every other domain object's deactivation convention in this
// codebase -- one-way, no reactivate) and immediately revokes any
// session it already holds, so a deactivation actually takes effect
// right away rather than merely blocking the *next* login while an
// existing session keeps working for up to sessionTTL. Refuses to let a
// platform admin deactivate their own account: there's no legitimate
// reason to self-lock via this endpoint (sign out instead), and the
// alternative is a confusing accidental lockout with no recovery path
// short of cmd/admin.
func (s *Server) handleDeactivateUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	callerID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}
	if userID == callerID {
		writeError(w, http.StatusBadRequest, "cannot deactivate your own account")
		return
	}

	txErr := s.withTx(r.Context(), func(q db.Querier) error {
		if _, err := q.DeactivateUser(r.Context(), userID); err != nil {
			return err
		}
		return q.RevokeAllSessionsForUser(r.Context(), userID)
	})
	if txErr != nil {
		s.writeDBError(w, txErr)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &callerID,
		Action:      auth.ActionUserDeactivated,
		EntityType:  ptr("user"),
		EntityID:    &userID,
	})

	w.WriteHeader(http.StatusNoContent)
}

type setPlatformAdminRequest struct {
	IsPlatformAdmin bool `json:"is_platform_admin"`
}

// handleSetPlatformAdmin grants or revokes platform-admin on an
// *existing* account -- handleCreateUser only covers granting it at
// creation time. Same self-target guard as handleDeactivateUser and for
// the same reason: revoking your own admin access by accident (or
// granting it, which is at least harmless) is a foot-gun with no
// legitimate use here, since another admin (or cmd/admin) can always do
// it instead.
func (s *Server) handleSetPlatformAdmin(w http.ResponseWriter, r *http.Request) {
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	callerID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}
	if userID == callerID {
		writeError(w, http.StatusBadRequest, "cannot change your own platform-admin status")
		return
	}

	var req setPlatformAdminRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if _, err := s.queries.SetUserPlatformAdmin(r.Context(), db.SetUserPlatformAdminParams{
		ID:              userID,
		IsPlatformAdmin: req.IsPlatformAdmin,
	}); err != nil {
		s.writeDBError(w, err)
		return
	}

	action := auth.ActionUserPlatformAdminRevoked
	if req.IsPlatformAdmin {
		action = auth.ActionUserPlatformAdminGranted
	}
	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &callerID,
		Action:      action,
		EntityType:  ptr("user"),
		EntityID:    &userID,
	})

	w.WriteHeader(http.StatusNoContent)
}

// sendInviteEmail emails a password-set link for token, which the caller
// has already generated (and, for a fresh account, committed alongside
// the user row -- see handleCreateUser). This is deliberately the only
// step of account creation that can still fail silently from the
// caller's perspective: it's a real network call to an external system,
// so it can't run inside a DB transaction the way token creation can,
// and unlike a DB error, isn't really actionable within the request
// itself. Reports whether it succeeded so callers can surface that (an
// invite_email_sent field on the response) rather than letting a 201
// imply the person actually received something -- handleResendInvite is
// the recovery path when this returns false.
func (s *Server) sendInviteEmail(ctx context.Context, user db.User, token string) bool {
	link := fmt.Sprintf("%s/set-password?token=%s", s.appBaseURL, token)
	msg := mail.Message{
		To:      user.Email,
		Subject: "Set up your CogLab Manager account",
		Body: fmt.Sprintf(
			"Hi %s,\n\nAn account has been created for you on CogLab Manager. "+
				"Set your password to get started:\n\n%s\n\nThis link expires in 7 days.\n",
			user.FirstName, link,
		),
	}
	if err := s.mailer.Send(ctx, msg); err != nil {
		s.logger.Error("send invite email", "user_id", user.ID, "error", err)
		return false
	}
	return true
}

type setPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// handleSetPassword redeems an invite token and, on success, logs the
// person straight in -- same response shape handleLogin returns, so the
// frontend's existing post-login flow (App.tsx's onLogin callback)
// handles it verbatim. Public (outside requireAuth): the person has no
// session yet, only the link from their invite email.
//
// Runs inside a transaction: auth.RedeemInviteToken's claim and its
// password write need to commit-or-rollback together, or a failure
// between the two (see RedeemInviteToken's doc comment) would burn the
// token without ever setting a password.
func (s *Server) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	var req setPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "token is required and password must be at least 8 characters")
		return
	}

	var identity auth.Identity
	txErr := s.withTx(r.Context(), func(q db.Querier) error {
		var err error
		identity, err = auth.RedeemInviteToken(r.Context(), q, req.Token, req.Password)
		return err
	})
	if txErr != nil {
		switch {
		case errors.Is(txErr, auth.ErrInvalidInviteToken):
			writeError(w, http.StatusBadRequest, "this link is invalid or has expired")
		case errors.Is(txErr, auth.ErrAccountDeactivated):
			writeError(w, http.StatusBadRequest, "this link is invalid or has expired")
		default:
			s.logger.Error("redeem invite token", "error", txErr)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
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
		Action:      auth.ActionPasswordSet,
	})

	writeJSON(w, http.StatusOK, loginResponse{User: userResponse{
		ID:              identity.UserID,
		Email:           identity.Email,
		FirstName:       identity.FirstName,
		LastName:        identity.LastName,
		IsPlatformAdmin: identity.IsPlatformAdmin,
	}})
}
