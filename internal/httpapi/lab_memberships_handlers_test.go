package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleListLabMemberships_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListLabMembershipsForLabFunc: func(ctx context.Context, labID int64) ([]db.ListLabMembershipsForLabRow, error) {
			return []db.ListLabMembershipsForLabRow{
				{UserID: 1, FirstName: "Pat", LastName: "Lee", Email: "pat@example.edu", RoleID: 1, RoleName: "staff", Priority: "undergrad_no_project"},
			}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]labMembershipResponse](t, rec)
	if len(got) != 1 || got[0].FirstName != "Pat" || got[0].RoleName != "staff" || got[0].Priority != "undergrad_no_project" {
		t.Errorf("memberships = %+v", got)
	}
}

func TestHandleListLabMemberships_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		ListLabMembershipsForLabFunc: func(ctx context.Context, labID int64) ([]db.ListLabMembershipsForLabRow, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestHandleListLabMemberships_NotAdminStillAllowed proves viewing the
// roster stays at plain-membership level -- only the mutating routes
// below require admin.
func TestHandleListLabMemberships_NotAdminStillAllowed(t *testing.T) {
	q := &dbfake.Querier{
		ListLabMembershipsForLabFunc: func(ctx context.Context, labID int64) ([]db.ListLabMembershipsForLabRow, error) {
			return nil, nil
		},
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) {
			t.Fatal("IsLabAdmin should not be called for a plain list request")
			return false, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}

func TestHandleCreateLabMembership_Success(t *testing.T) {
	var captured db.CreateLabMembershipParams
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		CreateLabMembershipFunc: func(ctx context.Context, arg db.CreateLabMembershipParams) (db.LabMembership, error) {
			captured = arg
			return db.LabMembership{ID: 5, UserID: arg.UserID, LabID: arg.LabID, RoleID: arg.RoleID, Priority: "undergrad_no_project"}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/", cookie, createLabMembershipRequest{UserID: 3, RoleID: 1})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.UserID != 3 || captured.LabID != 9 || captured.RoleID != 1 {
		t.Errorf("CreateLabMembership params = %+v", captured)
	}
}

// TestHandleCreateLabMembership_RequiresAdmin covers the privilege-
// escalation gap from code review: an ordinary (non-admin) lab member
// must not be able to add members or grant roles -- requireLabAdminFromURL
// rejects the request before CreateLabMembership is ever called.
func TestHandleCreateLabMembership_RequiresAdmin(t *testing.T) {
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return false, nil },
		CreateLabMembershipFunc: func(ctx context.Context, arg db.CreateLabMembershipParams) (db.LabMembership, error) {
			t.Fatal("CreateLabMembership should not be called when the caller isn't an admin")
			return db.LabMembership{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/", cookie, createLabMembershipRequest{UserID: 3, RoleID: 1})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestHandleCreateLabMembership_AlreadyMember covers re-adding someone
// already a member: the table's unique(user_id, lab_id) constraint
// rejects it, surfaced as a 409 by the existing writeDBError conflict
// handling.
func TestHandleCreateLabMembership_AlreadyMember(t *testing.T) {
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		CreateLabMembershipFunc: func(ctx context.Context, arg db.CreateLabMembershipParams) (db.LabMembership, error) {
			return db.LabMembership{}, &pgconn.PgError{Code: pgUniqueViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/", cookie, createLabMembershipRequest{UserID: 3, RoleID: 1})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandleUpdateLabMembership_Success(t *testing.T) {
	var captured db.UpdateLabMembershipParams
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		UpdateLabMembershipFunc: func(ctx context.Context, arg db.UpdateLabMembershipParams) (db.LabMembership, error) {
			captured = arg
			return db.LabMembership{ID: 5, UserID: arg.UserID, LabID: arg.LabID, RoleID: arg.RoleID, Priority: arg.Priority}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/labs/9/memberships/3/", cookie, updateLabMembershipRequest{RoleID: 2, Priority: "lab_director"})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.UserID != 3 || captured.LabID != 9 || captured.RoleID != 2 || captured.Priority != "lab_director" {
		t.Errorf("UpdateLabMembership params = %+v", captured)
	}
}

// TestHandleUpdateLabMembership_RequiresAdmin mirrors
// TestHandleCreateLabMembership_RequiresAdmin: an ordinary member must
// not be able to change anyone's permission role or priority -- not
// even their own (PUT /labs/9/memberships/7/ as user 7 would otherwise
// let a staff member self-promote to admin).
func TestHandleUpdateLabMembership_RequiresAdmin(t *testing.T) {
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return false, nil },
		UpdateLabMembershipFunc: func(ctx context.Context, arg db.UpdateLabMembershipParams) (db.LabMembership, error) {
			t.Fatal("UpdateLabMembership should not be called when the caller isn't an admin")
			return db.LabMembership{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/labs/9/memberships/7/", cookie, updateLabMembershipRequest{RoleID: 3, Priority: "lab_director"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestHandleUpdateLabMembership_NotFound covers a user who isn't
// actually a member of this lab: UpdateLabMembership's WHERE clause
// matches no row, so pgx.ErrNoRows becomes a 404 via writeDBError.
func TestHandleUpdateLabMembership_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		UpdateLabMembershipFunc: func(ctx context.Context, arg db.UpdateLabMembershipParams) (db.LabMembership, error) {
			return db.LabMembership{}, pgx.ErrNoRows
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPut, "/labs/9/memberships/3/", cookie, updateLabMembershipRequest{RoleID: 2, Priority: "lab_director"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleRemoveLabMembership_Success(t *testing.T) {
	var capturedRemove db.RemoveLabMembershipParams
	var capturedTrainings db.RemoveLabMemberTrainingsForUserInLabParams
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		RemoveLabMembershipFunc: func(ctx context.Context, arg db.RemoveLabMembershipParams) (db.LabMembership, error) {
			capturedRemove = arg
			return db.LabMembership{ID: 5, UserID: arg.UserID, LabID: arg.LabID}, nil
		},
		RemoveLabMemberTrainingsForUserInLabFunc: func(ctx context.Context, arg db.RemoveLabMemberTrainingsForUserInLabParams) error {
			capturedTrainings = arg
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/labs/9/memberships/3/", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if capturedRemove.UserID != 3 || capturedRemove.LabID != 9 {
		t.Errorf("RemoveLabMembership params = %+v", capturedRemove)
	}
	// The removed lab's trainings must be cleared too (code review: a
	// removed member's trainings previously survived, leaving them a
	// still-schedulable candidate for a lab they no longer belong to).
	if capturedTrainings.UserID != 3 || capturedTrainings.LabID != 9 {
		t.Errorf("RemoveLabMemberTrainingsForUserInLab params = %+v, want the same user/lab as the removed membership", capturedTrainings)
	}
}

// TestHandleRemoveLabMembership_RequiresAdmin mirrors the other
// mutating routes' admin gate.
func TestHandleRemoveLabMembership_RequiresAdmin(t *testing.T) {
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return false, nil },
		RemoveLabMembershipFunc: func(ctx context.Context, arg db.RemoveLabMembershipParams) (db.LabMembership, error) {
			t.Fatal("RemoveLabMembership should not be called when the caller isn't an admin")
			return db.LabMembership{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/labs/9/memberships/3/", cookie, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestHandleRemoveLabMembership_NotFound covers code review's "DELETE
// reports success when nothing was deleted" finding: RemoveLabMembership
// is now :one (RETURNING), so a membership that never existed surfaces
// pgx.ErrNoRows -> 404, and neither the trainings cleanup nor the audit
// event run (withTx rolls back on the first error).
func TestHandleRemoveLabMembership_NotFound(t *testing.T) {
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		RemoveLabMembershipFunc: func(ctx context.Context, arg db.RemoveLabMembershipParams) (db.LabMembership, error) {
			return db.LabMembership{}, pgx.ErrNoRows
		},
		RemoveLabMemberTrainingsForUserInLabFunc: func(ctx context.Context, arg db.RemoveLabMemberTrainingsForUserInLabParams) error {
			t.Fatal("trainings cleanup should not run when the membership delete found no row")
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			t.Fatal("no audit event should be recorded when the membership delete found no row")
			return db.AuditEvent{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/labs/9/memberships/3/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleRemoveLabMembership_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		RemoveLabMembershipFunc: func(ctx context.Context, arg db.RemoveLabMembershipParams) (db.LabMembership, error) {
			return db.LabMembership{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/labs/9/memberships/3/", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleSearchUsersNotInLab_Success(t *testing.T) {
	var captured db.SearchUsersNotInLabParams
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		SearchUsersNotInLabFunc: func(ctx context.Context, arg db.SearchUsersNotInLabParams) ([]db.User, error) {
			captured = arg
			return []db.User{{ID: 4, FirstName: "Jordan", LastName: "Blake", Email: "jordan@example.edu"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/search?q=jordan", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if captured.LabID != 9 || captured.NameQuery == nil || *captured.NameQuery != "jordan" {
		t.Errorf("SearchUsersNotInLab params = %+v", captured)
	}
	got := decodeBody[[]searchedUserResponse](t, rec)
	if len(got) != 1 || got[0].Email != "jordan@example.edu" {
		t.Errorf("results = %+v", got)
	}
}

// TestHandleSearchUsersNotInLab_RequiresAdmin mirrors the other
// mutating/browsing routes' admin gate -- this is the first line of
// defense against the "browse the institution-wide directory" finding
// from code review.
func TestHandleSearchUsersNotInLab_RequiresAdmin(t *testing.T) {
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return false, nil },
		SearchUsersNotInLabFunc: func(ctx context.Context, arg db.SearchUsersNotInLabParams) ([]db.User, error) {
			t.Fatal("SearchUsersNotInLab should not be called when the caller isn't an admin")
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/search?q=jordan", cookie, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestHandleSearchUsersNotInLab_EmptyQuery covers code review's second
// line of defense: even for an admin, an empty/missing q would (per
// SearchUsersNotInLab's own SQL) match every active user in the system,
// not just candidates matching a search -- rejected before the query
// runs at all.
func TestHandleSearchUsersNotInLab_EmptyQuery(t *testing.T) {
	q := &dbfake.Querier{
		IsLabAdminFunc: func(ctx context.Context, arg db.IsLabAdminParams) (bool, error) { return true, nil },
		SearchUsersNotInLabFunc: func(ctx context.Context, arg db.SearchUsersNotInLabParams) ([]db.User, error) {
			t.Fatal("SearchUsersNotInLab should not be called for an empty query")
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	for _, path := range []string{"/labs/9/memberships/search", "/labs/9/memberships/search?q=", "/labs/9/memberships/search?q=%20%20"} {
		rec := doRequest(t, s, http.MethodGet, path, cookie, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("path %q: status = %d, want %d", path, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestHandleListLabMemberTrainingsForUser_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListLabMemberTrainingsForUserFunc: func(ctx context.Context, arg db.ListLabMemberTrainingsForUserParams) ([]db.ExperimentRole, error) {
			if arg.UserID != 3 || arg.LabID != 9 {
				t.Fatalf("ListLabMemberTrainingsForUser params = %+v, want user 3 in lab 9", arg)
			}
			return []db.ExperimentRole{{ID: 11, LabID: 9, Name: "Experimenter"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/3/trainings", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]experimentRoleResponse](t, rec)
	if len(got) != 1 || got[0].Name != "Experimenter" {
		t.Errorf("trainings = %+v", got)
	}
}

// TestHandleListLabMemberTrainingsForUser_TargetNotInLab covers the
// cross-lab data leak from code review: the outer middleware only
// confirms the CALLER (user 7) belongs to lab 9, not that the requested
// {userID} (3) does. GetLabMembership is stubbed to succeed only for
// the caller, so the handler's own membership check on the target user
// must be what produces the 404 here.
func TestHandleListLabMemberTrainingsForUser_TargetNotInLab(t *testing.T) {
	q := &dbfake.Querier{
		GetLabMembershipFunc: func(ctx context.Context, arg db.GetLabMembershipParams) (db.LabMembership, error) {
			if arg.UserID == 7 {
				return db.LabMembership{UserID: 7, LabID: arg.LabID}, nil
			}
			return db.LabMembership{}, pgx.ErrNoRows
		},
		ListLabMemberTrainingsForUserFunc: func(ctx context.Context, arg db.ListLabMemberTrainingsForUserParams) ([]db.ExperimentRole, error) {
			t.Fatal("ListLabMemberTrainingsForUser should not be called when the target user isn't in this lab")
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/3/trainings", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
