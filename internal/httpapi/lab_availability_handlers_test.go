package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleCreateLabAvailabilityGeneral_Success(t *testing.T) {
	var captured db.CreateLabAvailabilityGeneralParams
	q := &dbfake.Querier{
		CreateLabAvailabilityGeneralFunc: func(ctx context.Context, arg db.CreateLabAvailabilityGeneralParams) (db.LabAvailabilityGeneral, error) {
			captured = arg
			return db.LabAvailabilityGeneral{ID: 1, UserID: arg.UserID, LabID: arg.LabID, Weekday: arg.Weekday, StartTime: arg.StartTime, EndTime: arg.EndTime}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/availability/general/", cookie, labAvailabilityGeneralRequest{
		Weekday: 1, StartTime: "09:00", EndTime: "17:00",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 || captured.Weekday != 1 {
		t.Errorf("CreateLabAvailabilityGeneral params = %+v", captured)
	}
	got := decodeBody[labAvailabilityGeneralResponse](t, rec)
	if got.StartTime != "09:00" || got.EndTime != "17:00" {
		t.Errorf("response = %+v", got)
	}
}

func TestHandleCreateLabAvailabilityGeneral_InvalidStartTime(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/availability/general/", cookie, labAvailabilityGeneralRequest{
		Weekday: 1, StartTime: "not-a-time", EndTime: "17:00",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListLabAvailabilityGeneral_Success(t *testing.T) {
	q := &dbfake.Querier{
		ListLabAvailabilityGeneralByUserFunc: func(ctx context.Context, arg db.ListLabAvailabilityGeneralByUserParams) ([]db.LabAvailabilityGeneral, error) {
			if arg.UserID != 7 {
				t.Errorf("ListLabAvailabilityGeneralByUser called with UserID=%d, want 7 (the current user)", arg.UserID)
			}
			return []db.LabAvailabilityGeneral{{ID: 1, UserID: 7, LabID: 9}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/availability/general/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}

func TestHandleDeactivateLabAvailabilityGeneral_Success(t *testing.T) {
	var deactivatedID int64
	q := &dbfake.Querier{
		GetLabAvailabilityGeneralByIDFunc: func(ctx context.Context, id int64) (db.LabAvailabilityGeneral, error) {
			return db.LabAvailabilityGeneral{ID: id, UserID: 7, LabID: 1}, nil
		},
		DeactivateLabAvailabilityGeneralFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/availability/general/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if deactivatedID != 3 {
		t.Errorf("deactivated ID = %d, want 3", deactivatedID)
	}
}

func TestHandleDeactivateLabAvailabilityGeneral_NotOwnedByCurrentUser(t *testing.T) {
	q := &dbfake.Querier{
		GetLabAvailabilityGeneralByIDFunc: func(ctx context.Context, id int64) (db.LabAvailabilityGeneral, error) {
			return db.LabAvailabilityGeneral{ID: id, UserID: 999, LabID: 1}, nil // belongs to someone else
		},
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return false, nil // plain staff, not a coordinator/admin of the row's lab
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/availability/general/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d -- another user's row should look like it doesn't exist", rec.Code, http.StatusNotFound)
	}
}

// TestHandleDeactivateLabAvailabilityGeneral_CoordinatorCanDeactivateForMember
// proves the on-behalf-of relaxation: a coordinator/admin of the row's lab
// can deactivate someone else's availability, unlike plain staff above.
func TestHandleDeactivateLabAvailabilityGeneral_CoordinatorCanDeactivateForMember(t *testing.T) {
	var deactivatedID int64
	var auditAction string
	q := &dbfake.Querier{
		GetLabAvailabilityGeneralByIDFunc: func(ctx context.Context, id int64) (db.LabAvailabilityGeneral, error) {
			return db.LabAvailabilityGeneral{ID: id, UserID: 999, LabID: 1}, nil
		},
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return true, nil
		},
		DeactivateLabAvailabilityGeneralFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			auditAction = arg.Action
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/availability/general/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 3 {
		t.Fatalf("deactivated id = %d, want 3", deactivatedID)
	}
	if auditAction != ActionLabAvailabilityManagedForMember {
		t.Fatalf("audit action = %q, want %q", auditAction, ActionLabAvailabilityManagedForMember)
	}
}

func TestHandleCreateLabAvailabilitySpecific_Success(t *testing.T) {
	var captured db.CreateLabAvailabilitySpecificParams
	q := &dbfake.Querier{
		CreateLabAvailabilitySpecificFunc: func(ctx context.Context, arg db.CreateLabAvailabilitySpecificParams) (db.LabAvailabilitySpecific, error) {
			captured = arg
			return db.LabAvailabilitySpecific{ID: 1, UserID: arg.UserID, LabID: arg.LabID, Date: arg.Date, StartTime: arg.StartTime, EndTime: arg.EndTime}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/availability/specific/", cookie, labAvailabilitySpecificRequest{
		Date: "2026-09-01", StartTime: "10:00", EndTime: "12:00",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.UserID != 7 || captured.LabID != 9 {
		t.Errorf("CreateLabAvailabilitySpecific params = %+v", captured)
	}
}

func TestHandleCreateLabAvailabilitySpecific_InvalidDate(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/availability/specific/", cookie, labAvailabilitySpecificRequest{
		Date: "not-a-date", StartTime: "10:00", EndTime: "12:00",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDeactivateLabAvailabilitySpecific_NotOwnedByCurrentUser(t *testing.T) {
	q := &dbfake.Querier{
		GetLabAvailabilitySpecificByIDFunc: func(ctx context.Context, id int64) (db.LabAvailabilitySpecific, error) {
			return db.LabAvailabilitySpecific{ID: id, UserID: 999, LabID: 1}, nil
		},
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return false, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/availability/specific/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDeactivateLabAvailabilitySpecific_CoordinatorCanDeactivateForMember(t *testing.T) {
	var deactivatedID int64
	var auditAction string
	q := &dbfake.Querier{
		GetLabAvailabilitySpecificByIDFunc: func(ctx context.Context, id int64) (db.LabAvailabilitySpecific, error) {
			return db.LabAvailabilitySpecific{ID: id, UserID: 999, LabID: 1}, nil
		},
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return true, nil
		},
		DeactivateLabAvailabilitySpecificFunc: func(ctx context.Context, id int64) error {
			deactivatedID = id
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			auditAction = arg.Action
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/availability/specific/3/deactivate", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if deactivatedID != 3 {
		t.Fatalf("deactivated id = %d, want 3", deactivatedID)
	}
	if auditAction != ActionLabAvailabilityManagedForMember {
		t.Fatalf("audit action = %q, want %q", auditAction, ActionLabAvailabilityManagedForMember)
	}
}

func TestHandleListLabAvailabilityGeneralForUser_Success(t *testing.T) {
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return true, nil
		},
		ListLabAvailabilityGeneralByUserFunc: func(ctx context.Context, arg db.ListLabAvailabilityGeneralByUserParams) ([]db.LabAvailabilityGeneral, error) {
			if arg.UserID != 3 {
				t.Errorf("ListLabAvailabilityGeneralByUser called with UserID=%d, want 3 (the target member, not the caller)", arg.UserID)
			}
			return []db.LabAvailabilityGeneral{{ID: 1, UserID: 3, LabID: 9}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/3/availability/general/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}

// TestHandleListLabAvailabilityGeneralForUser_RequiresCoordinatorOrAdmin
// proves a plain staff member can't view another member's schedule via
// this route -- requireLabCoordinatorOrAdminFromURL rejects the request
// before the query ever runs.
func TestHandleListLabAvailabilityGeneralForUser_RequiresCoordinatorOrAdmin(t *testing.T) {
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return false, nil
		},
		ListLabAvailabilityGeneralByUserFunc: func(ctx context.Context, arg db.ListLabAvailabilityGeneralByUserParams) ([]db.LabAvailabilityGeneral, error) {
			t.Fatal("ListLabAvailabilityGeneralByUser should not be called when the caller isn't a coordinator or admin")
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/3/availability/general/", cookie, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestHandleListLabAvailabilityGeneralForUser_TargetNotInLab covers the
// cross-lab data leak from code review: requireLabCoordinatorOrAdminFromURL
// only confirms the CALLER (user 7) is a coordinator/admin of labID, not
// that the requested {userID} (3) is even a member of it.
// GetLabMembership is stubbed to succeed only for the caller, so the
// handler's own membership check on the target user must be what
// produces the 404 here.
func TestHandleListLabAvailabilityGeneralForUser_TargetNotInLab(t *testing.T) {
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return true, nil
		},
		GetLabMembershipFunc: func(ctx context.Context, arg db.GetLabMembershipParams) (db.LabMembership, error) {
			if arg.UserID == 7 {
				return db.LabMembership{UserID: 7, LabID: arg.LabID}, nil
			}
			return db.LabMembership{}, pgx.ErrNoRows
		},
		ListLabAvailabilityGeneralByUserFunc: func(ctx context.Context, arg db.ListLabAvailabilityGeneralByUserParams) ([]db.LabAvailabilityGeneral, error) {
			t.Fatal("ListLabAvailabilityGeneralByUser should not be called when the target user isn't in this lab")
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/3/availability/general/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleCreateLabAvailabilityGeneralForUser_TargetNotInLab(t *testing.T) {
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return true, nil
		},
		GetLabMembershipFunc: func(ctx context.Context, arg db.GetLabMembershipParams) (db.LabMembership, error) {
			if arg.UserID == 7 {
				return db.LabMembership{UserID: 7, LabID: arg.LabID}, nil
			}
			return db.LabMembership{}, pgx.ErrNoRows
		},
		CreateLabAvailabilityGeneralFunc: func(ctx context.Context, arg db.CreateLabAvailabilityGeneralParams) (db.LabAvailabilityGeneral, error) {
			t.Fatal("CreateLabAvailabilityGeneral should not be called when the target user isn't in this lab")
			return db.LabAvailabilityGeneral{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/3/availability/general/", cookie, labAvailabilityGeneralRequest{
		Weekday: 1, StartTime: "09:00", EndTime: "17:00",
	})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleCreateLabAvailabilityGeneralForUser_Success(t *testing.T) {
	var captured db.CreateLabAvailabilityGeneralParams
	var auditAction string
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return true, nil
		},
		CreateLabAvailabilityGeneralFunc: func(ctx context.Context, arg db.CreateLabAvailabilityGeneralParams) (db.LabAvailabilityGeneral, error) {
			captured = arg
			return db.LabAvailabilityGeneral{ID: 1, UserID: arg.UserID, LabID: arg.LabID, Weekday: arg.Weekday, StartTime: arg.StartTime, EndTime: arg.EndTime}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			auditAction = arg.Action
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/3/availability/general/", cookie, labAvailabilityGeneralRequest{
		Weekday: 1, StartTime: "09:00", EndTime: "17:00",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.UserID != 3 || captured.LabID != 9 {
		t.Errorf("CreateLabAvailabilityGeneral params = %+v, want the target member (3), not the caller (7)", captured)
	}
	if auditAction != ActionLabAvailabilityManagedForMember {
		t.Errorf("audit action = %q, want %q", auditAction, ActionLabAvailabilityManagedForMember)
	}
}

func TestHandleCreateLabAvailabilityGeneralForUser_RequiresCoordinatorOrAdmin(t *testing.T) {
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return false, nil
		},
		CreateLabAvailabilityGeneralFunc: func(ctx context.Context, arg db.CreateLabAvailabilityGeneralParams) (db.LabAvailabilityGeneral, error) {
			t.Fatal("CreateLabAvailabilityGeneral should not be called when the caller isn't a coordinator or admin")
			return db.LabAvailabilityGeneral{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/3/availability/general/", cookie, labAvailabilityGeneralRequest{
		Weekday: 1, StartTime: "09:00", EndTime: "17:00",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleListLabAvailabilitySpecificForUser_Success(t *testing.T) {
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return true, nil
		},
		ListLabAvailabilitySpecificByUserFunc: func(ctx context.Context, arg db.ListLabAvailabilitySpecificByUserParams) ([]db.LabAvailabilitySpecific, error) {
			if arg.UserID != 3 {
				t.Errorf("ListLabAvailabilitySpecificByUser called with UserID=%d, want 3", arg.UserID)
			}
			return []db.LabAvailabilitySpecific{{ID: 1, UserID: 3, LabID: 9}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/3/availability/specific/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
}

func TestHandleListLabAvailabilitySpecificForUser_RequiresCoordinatorOrAdmin(t *testing.T) {
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return false, nil
		},
		ListLabAvailabilitySpecificByUserFunc: func(ctx context.Context, arg db.ListLabAvailabilitySpecificByUserParams) ([]db.LabAvailabilitySpecific, error) {
			t.Fatal("ListLabAvailabilitySpecificByUser should not be called when the caller isn't a coordinator or admin")
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/3/availability/specific/", cookie, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleListLabAvailabilitySpecificForUser_TargetNotInLab(t *testing.T) {
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return true, nil
		},
		GetLabMembershipFunc: func(ctx context.Context, arg db.GetLabMembershipParams) (db.LabMembership, error) {
			if arg.UserID == 7 {
				return db.LabMembership{UserID: 7, LabID: arg.LabID}, nil
			}
			return db.LabMembership{}, pgx.ErrNoRows
		},
		ListLabAvailabilitySpecificByUserFunc: func(ctx context.Context, arg db.ListLabAvailabilitySpecificByUserParams) ([]db.LabAvailabilitySpecific, error) {
			t.Fatal("ListLabAvailabilitySpecificByUser should not be called when the target user isn't in this lab")
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/memberships/3/availability/specific/", cookie, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleCreateLabAvailabilitySpecificForUser_TargetNotInLab(t *testing.T) {
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return true, nil
		},
		GetLabMembershipFunc: func(ctx context.Context, arg db.GetLabMembershipParams) (db.LabMembership, error) {
			if arg.UserID == 7 {
				return db.LabMembership{UserID: 7, LabID: arg.LabID}, nil
			}
			return db.LabMembership{}, pgx.ErrNoRows
		},
		CreateLabAvailabilitySpecificFunc: func(ctx context.Context, arg db.CreateLabAvailabilitySpecificParams) (db.LabAvailabilitySpecific, error) {
			t.Fatal("CreateLabAvailabilitySpecific should not be called when the target user isn't in this lab")
			return db.LabAvailabilitySpecific{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/3/availability/specific/", cookie, labAvailabilitySpecificRequest{
		Date: "2026-09-01", StartTime: "10:00", EndTime: "12:00",
	})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleCreateLabAvailabilitySpecificForUser_Success(t *testing.T) {
	var captured db.CreateLabAvailabilitySpecificParams
	var auditAction string
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return true, nil
		},
		CreateLabAvailabilitySpecificFunc: func(ctx context.Context, arg db.CreateLabAvailabilitySpecificParams) (db.LabAvailabilitySpecific, error) {
			captured = arg
			return db.LabAvailabilitySpecific{ID: 1, UserID: arg.UserID, LabID: arg.LabID, Date: arg.Date, StartTime: arg.StartTime, EndTime: arg.EndTime}, nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			auditAction = arg.Action
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/3/availability/specific/", cookie, labAvailabilitySpecificRequest{
		Date: "2026-09-01", StartTime: "10:00", EndTime: "12:00",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body)
	}
	if captured.UserID != 3 || captured.LabID != 9 {
		t.Errorf("CreateLabAvailabilitySpecific params = %+v, want the target member (3), not the caller (7)", captured)
	}
	if auditAction != ActionLabAvailabilityManagedForMember {
		t.Errorf("audit action = %q, want %q", auditAction, ActionLabAvailabilityManagedForMember)
	}
}

func TestHandleCreateLabAvailabilitySpecificForUser_RequiresCoordinatorOrAdmin(t *testing.T) {
	q := &dbfake.Querier{
		IsLabCoordinatorOrAdminFunc: func(ctx context.Context, arg db.IsLabCoordinatorOrAdminParams) (bool, error) {
			return false, nil
		},
		CreateLabAvailabilitySpecificFunc: func(ctx context.Context, arg db.CreateLabAvailabilitySpecificParams) (db.LabAvailabilitySpecific, error) {
			t.Fatal("CreateLabAvailabilitySpecific should not be called when the caller isn't a coordinator or admin")
			return db.LabAvailabilitySpecific{}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/labs/9/memberships/3/availability/specific/", cookie, labAvailabilitySpecificRequest{
		Date: "2026-09-01", StartTime: "10:00", EndTime: "12:00",
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
