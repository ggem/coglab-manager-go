package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/auth"
	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
	"github.com/ggem/coglab-manager-go/internal/mail/mailfake"
)

func futureTimestamptz() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}
}

// stubPlatformAdmin configures q so requirePlatformAdmin's GetUserByID
// lookup for userID reports IsPlatformAdmin -- newAuthenticatedTestServer
// already stubs the session; this only needs to add the user row.
func stubPlatformAdmin(q *dbfake.Querier, userID int64, isAdmin bool) {
	q.GetUserByIDFunc = func(ctx context.Context, id int64) (db.User, error) {
		return db.User{ID: id, IsPlatformAdmin: isAdmin}, nil
	}
}

func TestHandleListUsers_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListUsersFunc: func(ctx context.Context) ([]db.User, error) {
			hash := "a-hash"
			return []db.User{
				{ID: 1, Email: "a@example.edu", FirstName: "Ada", LastName: "Lovelace", PasswordHash: &hash},
				{ID: 2, Email: "b@example.edu", FirstName: "Grace", LastName: "Hopper", IsPlatformAdmin: true},
			}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/admin/users/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]adminUserResponse](t, rec)
	if len(got) != 2 || !got[0].HasPassword || got[1].HasPassword || !got[1].IsPlatformAdmin {
		t.Errorf("users = %+v", got)
	}
}

func TestHandleListUsers_RequiresPlatformAdmin(t *testing.T) {
	q := &dbfake.Querier{
		ListUsersFunc: func(ctx context.Context) ([]db.User, error) {
			t.Fatal("ListUsers should not be called when the caller isn't a platform admin")
			return nil, nil
		},
	}
	stubPlatformAdmin(q, 7, false)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/admin/users/", cookie, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleCreateUser_Success(t *testing.T) {
	var created db.CreateUserParams
	q := &dbfake.Querier{
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			created = arg
			return db.User{ID: 42, Email: arg.Email, FirstName: arg.FirstName, LastName: arg.LastName, IsPlatformAdmin: arg.IsPlatformAdmin}, nil
		},
		CreatePasswordSetTokenFunc: func(ctx context.Context, arg db.CreatePasswordSetTokenParams) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{ID: 1, TokenHash: arg.TokenHash, UserID: arg.UserID, ExpiresAt: arg.ExpiresAt}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	sender := &mailfake.Sender{}
	s, cookie := newAuthenticatedTestServer(q, 7)
	s.mailer = sender
	s.appBaseURL = "http://localhost:5173"

	rec := doRequest(t, s, http.MethodPost, "/admin/users/", cookie, createUserRequest{
		Email: "new-hire@example.edu", FirstName: "Barbara", LastName: "Liskov", IsPlatformAdmin: true,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if created.PasswordHash != nil {
		t.Error("a newly created user should have no password_hash")
	}
	if !created.IsPlatformAdmin {
		t.Error("expected IsPlatformAdmin to be honored for a platform-admin-created user")
	}
	got := decodeBody[createUserResponse](t, rec)
	if got.ID != 42 || got.HasPassword || !got.InviteEmailSent {
		t.Errorf("response = %+v", got)
	}

	sent := sender.Messages()
	if len(sent) != 1 || sent[0].To != "new-hire@example.edu" {
		t.Fatalf("Messages() = %+v, want one invite email to new-hire@example.edu", sent)
	}
}

// TestHandleCreateUser_WithLabAssignment_Success proves the platform-admin
// "assign a lab at creation" path: both lab_id and role_id set creates the
// membership in the same transaction as the user and records both audit
// events.
func TestHandleCreateUser_WithLabAssignment_Success(t *testing.T) {
	var capturedMembership db.CreateLabMembershipParams
	var auditActions []string
	q := &dbfake.Querier{
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			return db.User{ID: 42, Email: arg.Email, FirstName: arg.FirstName, LastName: arg.LastName}, nil
		},
		CreatePasswordSetTokenFunc: func(ctx context.Context, arg db.CreatePasswordSetTokenParams) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{ID: 1}, nil
		},
		CreateLabMembershipFunc: func(ctx context.Context, arg db.CreateLabMembershipParams) (db.LabMembership, error) {
			capturedMembership = arg
			return db.LabMembership{ID: 5, UserID: arg.UserID, LabID: arg.LabID, RoleID: arg.RoleID}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			auditActions = append(auditActions, arg.Action)
			return db.AuditEvent{ID: 1}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)
	s.mailer = &mailfake.Sender{}
	s.appBaseURL = "http://localhost:5173"

	labID, roleID := int64(9), int64(1)
	rec := doRequest(t, s, http.MethodPost, "/admin/users/", cookie, createUserRequest{
		Email: "new-hire@example.edu", FirstName: "Barbara", LastName: "Liskov",
		LabID: &labID, RoleID: &roleID,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if capturedMembership.UserID != 42 || capturedMembership.LabID != 9 || capturedMembership.RoleID != 1 {
		t.Errorf("CreateLabMembership params = %+v", capturedMembership)
	}
	if len(auditActions) != 2 || auditActions[0] != auth.ActionUserCreated || auditActions[1] != ActionLabMembershipCreated {
		t.Errorf("audit actions = %+v, want [%s %s]", auditActions, auth.ActionUserCreated, ActionLabMembershipCreated)
	}
}

// TestHandleCreateUser_LabIDWithoutRoleID_BadRequest and its RoleID
// counterpart below prove the both-or-neither validation: you can't
// assign a lab without a role, or vice versa.
func TestHandleCreateUser_LabIDWithoutRoleID_BadRequest(t *testing.T) {
	q := &dbfake.Querier{
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			t.Fatal("CreateUser should not be called when lab_id/role_id validation fails")
			return db.User{}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	labID := int64(9)
	rec := doRequest(t, s, http.MethodPost, "/admin/users/", cookie, createUserRequest{
		Email: "new-hire@example.edu", FirstName: "A", LastName: "B", LabID: &labID,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateUser_RoleIDWithoutLabID_BadRequest(t *testing.T) {
	q := &dbfake.Querier{
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			t.Fatal("CreateUser should not be called when lab_id/role_id validation fails")
			return db.User{}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	roleID := int64(1)
	rec := doRequest(t, s, http.MethodPost, "/admin/users/", cookie, createUserRequest{
		Email: "new-hire@example.edu", FirstName: "A", LastName: "B", RoleID: &roleID,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestHandleCreateUser_LabAssignmentFailureRollsBackUser proves an invalid
// lab_id/role_id (e.g. a foreign-key violation) fails the whole request
// cleanly rather than leaving an orphaned, lab-less account -- same
// transactional guarantee as TestHandleCreateUser_TokenCreationFailureRollsBackUser.
func TestHandleCreateUser_LabAssignmentFailureRollsBackUser(t *testing.T) {
	txQueries := &dbfake.Querier{
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			return db.User{ID: 42, Email: arg.Email, FirstName: arg.FirstName, LastName: arg.LastName}, nil
		},
		CreatePasswordSetTokenFunc: func(ctx context.Context, arg db.CreatePasswordSetTokenParams) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{ID: 1}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
		CreateLabMembershipFunc: func(ctx context.Context, arg db.CreateLabMembershipParams) (db.LabMembership, error) {
			return db.LabMembership{}, &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	stubPlatformAdmin(txQueries, 7, true)
	s, cookie := newAuthenticatedTestServer(txQueries, 7)

	labID, roleID := int64(999), int64(1)
	rec := doRequest(t, s, http.MethodPost, "/admin/users/", cookie, createUserRequest{
		Email: "new-hire@example.edu", FirstName: "A", LastName: "B", LabID: &labID, RoleID: &roleID,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body)
	}
}

func TestHandleCreateUser_RequiresPlatformAdmin(t *testing.T) {
	q := &dbfake.Querier{
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			t.Fatal("CreateUser should not be called when the caller isn't a platform admin")
			return db.User{}, nil
		},
	}
	stubPlatformAdmin(q, 7, false)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/", cookie, createUserRequest{
		Email: "new-hire@example.edu", FirstName: "Barbara", LastName: "Liskov",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleCreateUser_DuplicateEmail(t *testing.T) {
	q := &dbfake.Querier{
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			return db.User{}, &pgconn.PgError{Code: pgUniqueViolation}
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/", cookie, createUserRequest{
		Email: "dup@example.edu", FirstName: "A", LastName: "B",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

// TestHandleCreateUser_TokenCreationFailureRollsBackUser proves that a
// failure creating the invite token (a transient DB error, not a
// business rule) rolls back the user row too, since both run in the
// same transaction -- the caller can retry the identical request rather
// than being permanently stuck on a unique-email conflict against an
// account with no working invite.
func TestHandleCreateUser_TokenCreationFailureRollsBackUser(t *testing.T) {
	txQueries := &dbfake.Querier{
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			return db.User{ID: 42, Email: arg.Email, FirstName: arg.FirstName, LastName: arg.LastName}, nil
		},
		CreatePasswordSetTokenFunc: func(ctx context.Context, arg db.CreatePasswordSetTokenParams) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{}, assertErr("connection reset by peer")
		},
	}
	stubPlatformAdmin(txQueries, 7, true)
	s, cookie := newAuthenticatedTestServer(txQueries, 7)
	// beginner == nil (the default from newTestServer) makes withTx run
	// its callback directly against s.queries with no real rollback --
	// this test only needs to confirm the handler surfaces the failure
	// as an error response rather than a 201 with a half-created account,
	// which it does regardless; true rollback behavior is exercised
	// against a real Postgres in the integration suite.

	rec := doRequest(t, s, http.MethodPost, "/admin/users/", cookie, createUserRequest{
		Email: "new-hire@example.edu", FirstName: "A", LastName: "B",
	})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body)
	}
}

func TestHandleCreateUser_InviteEmailFailureReportsUnsent(t *testing.T) {
	// The account (and its invite token) are already committed by the
	// time the email is sent -- an SMTP failure shouldn't turn a
	// successful account creation into an error response, but it must
	// be visible in the response rather than an indistinguishable 201,
	// since there's no other way for the admin to know the person can't
	// yet get in (see handleResendInvite for the recovery path).
	q := &dbfake.Querier{
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			return db.User{ID: 1, Email: arg.Email, FirstName: arg.FirstName, LastName: arg.LastName}, nil
		},
		CreatePasswordSetTokenFunc: func(ctx context.Context, arg db.CreatePasswordSetTokenParams) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{ID: 1}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)
	s.mailer = &mailfake.Sender{Err: assertErr("smtp relay down")}

	rec := doRequest(t, s, http.MethodPost, "/admin/users/", cookie, createUserRequest{
		Email: "new-hire@example.edu", FirstName: "A", LastName: "B",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	got := decodeBody[createUserResponse](t, rec)
	if got.InviteEmailSent {
		t.Error("InviteEmailSent = true, want false when the mailer fails")
	}
}

func TestHandleResendInvite_Success(t *testing.T) {
	var created db.CreatePasswordSetTokenParams
	q := &dbfake.Querier{
		GetUserByIDFunc: func(ctx context.Context, id int64) (db.User, error) {
			if id == 7 {
				return db.User{ID: 7, IsPlatformAdmin: true}, nil
			}
			return db.User{ID: 42, Email: "new-hire@example.edu", FirstName: "Barbara"}, nil
		},
		CreatePasswordSetTokenFunc: func(ctx context.Context, arg db.CreatePasswordSetTokenParams) (db.PasswordSetToken, error) {
			created = arg
			return db.PasswordSetToken{ID: 2, TokenHash: arg.TokenHash, UserID: arg.UserID, ExpiresAt: arg.ExpiresAt}, nil
		},
	}
	sender := &mailfake.Sender{}
	s, cookie := newAuthenticatedTestServer(q, 7)
	s.mailer = sender
	s.appBaseURL = "http://localhost:5173"

	rec := doRequest(t, s, http.MethodPost, "/admin/users/42/resend-invite", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[resendInviteResponse](t, rec)
	if !got.InviteEmailSent {
		t.Error("InviteEmailSent = false, want true")
	}
	if created.UserID != 42 {
		t.Errorf("CreatePasswordSetToken UserID = %d, want 42", created.UserID)
	}
	sent := sender.Messages()
	if len(sent) != 1 || sent[0].To != "new-hire@example.edu" {
		t.Fatalf("Messages() = %+v, want one invite email to new-hire@example.edu", sent)
	}
}

func TestHandleResendInvite_AlreadyActivated(t *testing.T) {
	hash := "a-hash"
	q := &dbfake.Querier{
		GetUserByIDFunc: func(ctx context.Context, id int64) (db.User, error) {
			if id == 7 {
				return db.User{ID: 7, IsPlatformAdmin: true}, nil
			}
			return db.User{ID: 42, PasswordHash: &hash}, nil
		},
		CreatePasswordSetTokenFunc: func(ctx context.Context, arg db.CreatePasswordSetTokenParams) (db.PasswordSetToken, error) {
			t.Fatal("should not create an invite token for an already-activated account")
			return db.PasswordSetToken{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/42/resend-invite", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleResendInvite_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		GetUserByIDFunc: func(ctx context.Context, id int64) (db.User, error) {
			if id == 7 {
				return db.User{ID: 7, IsPlatformAdmin: true}, nil
			}
			return db.User{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/999/resend-invite", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleResendInvite_RequiresPlatformAdmin(t *testing.T) {
	q := &dbfake.Querier{
		GetUserByIDFunc: func(ctx context.Context, id int64) (db.User, error) {
			return db.User{ID: id, IsPlatformAdmin: false}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/42/resend-invite", cookie, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleSetPassword_Success(t *testing.T) {
	var capturedPassword db.SetUserPasswordParams
	var claimedHash []byte
	var capturedSession db.CreateSessionParams
	q := &dbfake.Querier{
		ClaimPasswordSetTokenFunc: func(ctx context.Context, tokenHash []byte) (db.PasswordSetToken, error) {
			claimedHash = tokenHash
			return db.PasswordSetToken{ID: 5, UserID: 42, ExpiresAt: futureTimestamptz(), UsedAt: futureTimestamptz()}, nil
		},
		GetUserByIDFunc: func(ctx context.Context, id int64) (db.User, error) {
			return db.User{ID: 42, Email: "new-hire@example.edu", FirstName: "Barbara", LastName: "Liskov"}, nil
		},
		SetUserPasswordFunc: func(ctx context.Context, arg db.SetUserPasswordParams) error {
			capturedPassword = arg
			return nil
		},
		CreateSessionFunc: func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
			capturedSession = arg
			return db.Session{ID: 1, UserID: arg.UserID, TokenHash: arg.TokenHash}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s := newTestServer(q)

	rec := doRequest(t, s, http.MethodPost, "/set-password", nil, setPasswordRequest{Token: "some-token", Password: "a-long-password"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[loginResponse](t, rec)
	if got.User.ID != 42 || got.User.Email != "new-hire@example.edu" {
		t.Errorf("response = %+v", got)
	}
	if capturedPassword.ID != 42 || capturedPassword.PasswordHash == nil {
		t.Errorf("SetUserPassword params = %+v", capturedPassword)
	}
	if len(claimedHash) == 0 {
		t.Error("ClaimPasswordSetToken called with an empty hash")
	}
	if capturedSession.UserID != 42 {
		t.Errorf("CreateSession UserID = %d, want 42", capturedSession.UserID)
	}
	if len(rec.Result().Cookies()) != 1 {
		t.Error("expected a session cookie to be set")
	}
}

func TestHandleSetPassword_InvalidToken(t *testing.T) {
	q := &dbfake.Querier{
		ClaimPasswordSetTokenFunc: func(ctx context.Context, tokenHash []byte) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{}, pgx.ErrNoRows
		},
	}
	s := newTestServer(q)

	rec := doRequest(t, s, http.MethodPost, "/set-password", nil, setPasswordRequest{Token: "bad-token", Password: "a-long-password"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSetPassword_PasswordTooShort(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/set-password", nil, setPasswordRequest{Token: "some-token", Password: "short"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateLabMembershipForNewUser_Success(t *testing.T) {
	var createdUser db.CreateUserParams
	var createdMembership db.CreateLabMembershipParams
	sender := &mailfake.Sender{}
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			createdUser = arg
			return db.User{ID: 99, Email: arg.Email, FirstName: arg.FirstName, LastName: arg.LastName}, nil
		},
		CreateLabMembershipFunc: func(ctx context.Context, arg db.CreateLabMembershipParams) (db.LabMembership, error) {
			createdMembership = arg
			return db.LabMembership{ID: 1, UserID: arg.UserID, LabID: arg.LabID, RoleID: arg.RoleID}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
		CreatePasswordSetTokenFunc: func(ctx context.Context, arg db.CreatePasswordSetTokenParams) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)
	s.mailer = sender
	s.appBaseURL = "http://localhost:5173"

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/new-user", cookie, createLabMembershipForNewUserRequest{
		Email: "new-hire@example.edu", FirstName: "Barbara", LastName: "Liskov", RoleID: 2,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if createdUser.Email != "new-hire@example.edu" || createdUser.IsPlatformAdmin {
		t.Errorf("CreateUser params = %+v", createdUser)
	}
	if createdMembership.UserID != 99 || createdMembership.LabID != 9 || createdMembership.RoleID != 2 {
		t.Errorf("CreateLabMembership params = %+v", createdMembership)
	}
	got := decodeBody[createdLabMemberResponse](t, rec)
	if !got.InviteEmailSent {
		t.Error("InviteEmailSent = false, want true")
	}
}

// TestHandleCreateLabMembershipForNewUser_CannotGrantPlatformAdmin pins
// down the privilege-escalation risk this endpoint must never reopen: a
// lab admin's authority is scoped to their own lab, and the request body
// has no is_platform_admin field for a client to even attempt to set --
// this test proves the server-side call always hardcodes false
// regardless of what a hand-crafted request might try to smuggle in via
// an unknown JSON field.
func TestHandleCreateLabMembershipForNewUser_CannotGrantPlatformAdmin(t *testing.T) {
	var createdUser db.CreateUserParams
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			createdUser = arg
			return db.User{ID: 99, Email: arg.Email}, nil
		},
		CreateLabMembershipFunc: func(ctx context.Context, arg db.CreateLabMembershipParams) (db.LabMembership, error) {
			return db.LabMembership{ID: 1, UserID: arg.UserID, LabID: arg.LabID, RoleID: arg.RoleID}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
		CreatePasswordSetTokenFunc: func(ctx context.Context, arg db.CreatePasswordSetTokenParams) (db.PasswordSetToken, error) {
			return db.PasswordSetToken{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/new-user", cookie, map[string]any{
		"email": "attacker@example.edu", "first_name": "A", "last_name": "B", "role_id": 2, "is_platform_admin": true,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if createdUser.IsPlatformAdmin {
		t.Fatal("a lab admin must never be able to grant platform-admin via this endpoint")
	}
}

func TestHandleCreateLabMembershipForNewUser_RequiresAdmin(t *testing.T) {
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return false, nil },
		CreateUserFunc: func(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
			t.Fatal("CreateUser should not be called when the caller isn't a lab admin")
			return db.User{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/new-user", cookie, createLabMembershipForNewUserRequest{
		Email: "new-hire@example.edu", FirstName: "A", LastName: "B", RoleID: 2,
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleDeactivateUser_Success(t *testing.T) {
	var deactivatedID int64 = -1
	var revokedSessionsUserID int64 = -1
	q := &dbfake.Querier{
		DeactivateUserFunc: func(ctx context.Context, id int64) (db.User, error) {
			deactivatedID = id
			return db.User{ID: id, DeactivatedAt: futureTimestamptz()}, nil
		},
		RevokeAllSessionsForUserFunc: func(ctx context.Context, userID int64) error {
			revokedSessionsUserID = userID
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/42/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if deactivatedID != 42 {
		t.Errorf("DeactivateUser called with %d, want 42", deactivatedID)
	}
	if revokedSessionsUserID != 42 {
		t.Errorf("RevokeAllSessionsForUser called with %d, want 42", revokedSessionsUserID)
	}
}

func TestHandleDeactivateUser_CannotDeactivateSelf(t *testing.T) {
	q := &dbfake.Querier{
		DeactivateUserFunc: func(ctx context.Context, id int64) (db.User, error) {
			t.Fatal("should not deactivate when the target is the caller")
			return db.User{}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/7/deactivate", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateUser_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		DeactivateUserFunc: func(ctx context.Context, id int64) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/999/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateUser_RequiresPlatformAdmin(t *testing.T) {
	q := &dbfake.Querier{
		DeactivateUserFunc: func(ctx context.Context, id int64) (db.User, error) {
			t.Fatal("DeactivateUser should not be called when the caller isn't a platform admin")
			return db.User{}, nil
		},
	}
	stubPlatformAdmin(q, 7, false)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/42/deactivate", cookie, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleSetPlatformAdmin_Grant(t *testing.T) {
	var captured db.SetUserPlatformAdminParams
	q := &dbfake.Querier{
		SetUserPlatformAdminFunc: func(ctx context.Context, arg db.SetUserPlatformAdminParams) (db.User, error) {
			captured = arg
			return db.User{ID: arg.ID, IsPlatformAdmin: arg.IsPlatformAdmin}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/42/platform-admin", cookie, setPlatformAdminRequest{IsPlatformAdmin: true})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.ID != 42 || !captured.IsPlatformAdmin {
		t.Errorf("SetUserPlatformAdmin params = %+v, want ID=42 IsPlatformAdmin=true", captured)
	}
}

func TestHandleSetPlatformAdmin_Revoke(t *testing.T) {
	var captured db.SetUserPlatformAdminParams
	q := &dbfake.Querier{
		SetUserPlatformAdminFunc: func(ctx context.Context, arg db.SetUserPlatformAdminParams) (db.User, error) {
			captured = arg
			return db.User{ID: arg.ID, IsPlatformAdmin: arg.IsPlatformAdmin}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/42/platform-admin", cookie, setPlatformAdminRequest{IsPlatformAdmin: false})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.ID != 42 || captured.IsPlatformAdmin {
		t.Errorf("SetUserPlatformAdmin params = %+v, want ID=42 IsPlatformAdmin=false", captured)
	}
}

func TestHandleSetPlatformAdmin_CannotChangeSelf(t *testing.T) {
	q := &dbfake.Querier{
		SetUserPlatformAdminFunc: func(ctx context.Context, arg db.SetUserPlatformAdminParams) (db.User, error) {
			t.Fatal("should not change platform-admin status when the target is the caller")
			return db.User{}, nil
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/7/platform-admin", cookie, setPlatformAdminRequest{IsPlatformAdmin: false})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSetPlatformAdmin_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		SetUserPlatformAdminFunc: func(ctx context.Context, arg db.SetUserPlatformAdminParams) (db.User, error) {
			return db.User{}, pgx.ErrNoRows
		},
	}
	stubPlatformAdmin(q, 7, true)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/999/platform-admin", cookie, setPlatformAdminRequest{IsPlatformAdmin: true})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleSetPlatformAdmin_RequiresPlatformAdmin(t *testing.T) {
	q := &dbfake.Querier{
		SetUserPlatformAdminFunc: func(ctx context.Context, arg db.SetUserPlatformAdminParams) (db.User, error) {
			t.Fatal("SetUserPlatformAdmin should not be called when the caller isn't a platform admin")
			return db.User{}, nil
		},
	}
	stubPlatformAdmin(q, 7, false)
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/admin/users/42/platform-admin", cookie, setPlatformAdminRequest{IsPlatformAdmin: true})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
