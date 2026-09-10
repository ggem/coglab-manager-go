package httpapi

import (
	"net/http"
	"strings"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/auth"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionLabMembershipCreated = "lab_membership.created"
	ActionLabMembershipUpdated = "lab_membership.updated"
	ActionLabMembershipRemoved = "lab_membership.removed"
)

type labMembershipResponse struct {
	UserID    int64  `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	RoleID    int64  `json:"role_id"`
	RoleName  string `json:"role_name"`
	Priority  string `json:"priority"`
}

func labMembershipRowToResponse(row db.ListLabMembershipsForLabRow) labMembershipResponse {
	return labMembershipResponse{
		UserID:    row.UserID,
		FirstName: row.FirstName,
		LastName:  row.LastName,
		Email:     row.Email,
		RoleID:    row.RoleID,
		RoleName:  row.RoleName,
		Priority:  row.Priority,
	}
}

// handleListLabMemberships is the lab-members admin page's roster --
// membership details (permission role, scheduling priority), not just
// the bare candidate-pool user rows handleListLabMembers returns.
func (s *Server) handleListLabMemberships(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	rows, err := s.queries.ListLabMembershipsForLab(r.Context(), labID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]labMembershipResponse, len(rows))
	for i, row := range rows {
		resp[i] = labMembershipRowToResponse(row)
	}
	writeJSON(w, http.StatusOK, resp)
}

type createLabMembershipRequest struct {
	UserID int64 `json:"user_id"`
	RoleID int64 `json:"role_id"`
}

// handleCreateLabMembership adds an existing user to a lab -- priority
// starts at the column's default and is tuned afterward via
// handleUpdateLabMembership. Re-adding someone already a member is
// rejected by the table's own unique(user_id, lab_id) constraint,
// surfaced as a 409 by the existing writeDBError conflict handling.
func (s *Server) handleCreateLabMembership(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req createLabMembershipRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	membership, err := s.queries.CreateLabMembership(r.Context(), db.CreateLabMembershipParams{
		UserID: req.UserID,
		LabID:  labID,
		RoleID: req.RoleID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &labID,
		Action:      ActionLabMembershipCreated,
		EntityType:  ptr("lab_membership"),
		EntityID:    &membership.ID,
		Metadata:    map[string]int64{"user_id": req.UserID, "role_id": req.RoleID},
	})

	w.WriteHeader(http.StatusNoContent)
}

type createLabMembershipForNewUserRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	RoleID    int64  `json:"role_id"`
}

// handleCreateLabMembershipForNewUser is the lab-admin counterpart to
// handleCreateUser: creates a brand-new account (for someone who's never
// used the app before, so isn't findable via handleSearchUsersNotInLab)
// and adds them to this lab in one step. Deliberately does NOT accept an
// is_platform_admin field -- unlike handleCreateUser, which only a
// platform admin can reach, a lab admin's authority is scoped to their
// own lab and must never extend to granting system-wide admin rights.
// User creation and the membership are one transaction: either both
// succeed or neither does, so a mid-way failure can't leave an orphaned
// account with no lab.
func (s *Server) handleCreateLabMembershipForNewUser(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req createLabMembershipForNewUserRequest
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
			IsPlatformAdmin: false,
		})
		if err != nil {
			return err
		}

		membership, err := q.CreateLabMembership(r.Context(), db.CreateLabMembershipParams{
			UserID: user.ID,
			LabID:  labID,
			RoleID: req.RoleID,
		})
		if err != nil {
			return err
		}

		// Also inside the transaction, same reasoning as
		// handleCreateUser: a token-creation failure here rolls back the
		// user and membership too, so the caller can just retry the
		// identical request instead of being stuck on a unique-email
		// conflict with an orphaned, un-invitable account.
		token, err = auth.CreateInviteToken(r.Context(), q, user.ID)
		if err != nil {
			return err
		}

		recorder := audit.NewRecorder(q)
		if err := recorder.Record(r.Context(), audit.Event{
			ActorUserID: currentUserID(r.Context()),
			LabID:       &labID,
			Action:      auth.ActionUserCreated,
			EntityType:  ptr("user"),
			EntityID:    &user.ID,
			Metadata:    map[string]any{"email": user.Email},
		}); err != nil {
			return err
		}
		return recorder.Record(r.Context(), audit.Event{
			ActorUserID: currentUserID(r.Context()),
			LabID:       &labID,
			Action:      ActionLabMembershipCreated,
			EntityType:  ptr("lab_membership"),
			EntityID:    &membership.ID,
			Metadata:    map[string]int64{"user_id": user.ID, "role_id": req.RoleID},
		})
	})
	if txErr != nil {
		s.writeDBError(w, txErr)
		return
	}

	sent := s.sendInviteEmail(r.Context(), user, token)

	writeJSON(w, http.StatusCreated, createdLabMemberResponse{
		searchedUserResponse{ID: user.ID, FirstName: user.FirstName, LastName: user.LastName, Email: user.Email},
		sent,
	})
}

type updateLabMembershipRequest struct {
	RoleID   int64  `json:"role_id"`
	Priority string `json:"priority"`
}

func (s *Server) handleUpdateLabMembership(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}

	var req updateLabMembershipRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	membership, err := s.queries.UpdateLabMembership(r.Context(), db.UpdateLabMembershipParams{
		UserID:   userID,
		LabID:    labID,
		RoleID:   req.RoleID,
		Priority: req.Priority,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &labID,
		Action:      ActionLabMembershipUpdated,
		EntityType:  ptr("lab_membership"),
		EntityID:    &membership.ID,
		Metadata:    map[string]any{"role_id": req.RoleID, "priority": req.Priority},
	})

	w.WriteHeader(http.StatusNoContent)
}

// handleRemoveLabMembership is a hard delete, not a deactivation --
// lab_memberships has no deactivated_at column. Runs in one transaction
// with RemoveLabMemberTrainingsForUserInLab: removing someone from a lab
// also retires them from that lab's studies, rather than leaving their
// trainings behind as a still-schedulable candidate. RemoveLabMembership
// is :one (returning the deleted row), not :exec, so a membership that
// didn't exist 404s instead of silently reporting success, and so the
// audit event can use the real lab_memberships.id -- consistent with
// Create/UpdateLabMembership -- instead of the user id.
func (s *Server) handleRemoveLabMembership(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}

	var removed db.LabMembership
	txErr := s.withTx(r.Context(), func(q db.Querier) error {
		var err error
		removed, err = q.RemoveLabMembership(r.Context(), db.RemoveLabMembershipParams{
			UserID: userID,
			LabID:  labID,
		})
		if err != nil {
			return err
		}
		if err := q.RemoveLabMemberTrainingsForUserInLab(r.Context(), db.RemoveLabMemberTrainingsForUserInLabParams{
			UserID: userID,
			LabID:  labID,
		}); err != nil {
			return err
		}
		return audit.NewRecorder(q).Record(r.Context(), audit.Event{
			ActorUserID: currentUserID(r.Context()),
			LabID:       &labID,
			Action:      ActionLabMembershipRemoved,
			EntityType:  ptr("lab_membership"),
			EntityID:    &removed.ID,
		})
	})
	if txErr != nil {
		s.writeDBError(w, txErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// searchedUserResponse is deliberately smaller than a full user
// response -- same rationale as trainedMemberResponse -- but keeps
// email, since disambiguating same-named staff is the whole point of
// this search.
type searchedUserResponse struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// createdLabMemberResponse is handleCreateLabMembershipForNewUser's
// response -- same invite_email_sent rationale as handleCreateUser's
// createUserResponse: a bare searchedUserResponse can't tell the caller
// whether the person will actually receive a way to activate the
// account it just created.
type createdLabMemberResponse struct {
	searchedUserResponse
	InviteEmailSent bool `json:"invite_email_sent"`
}

// handleSearchUsersNotInLab is the candidate pool for "add an existing
// person to this lab" -- active users who aren't already a member. Only
// a lab admin can call this (see requireLabAdminFromURL), and a
// non-empty query is required on top of that: SearchUsersNotInLab's own
// SQL treats a nil query as "match everyone", which combined with no
// query would turn this into an unbounded (if 20-at-a-time) directory
// browse of every active user in the system.
func (s *Server) handleSearchUsersNotInLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	q := queryString(r, "q")
	if q == nil || strings.TrimSpace(*q) == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}

	users, err := s.queries.SearchUsersNotInLab(r.Context(), db.SearchUsersNotInLabParams{
		NameQuery: q,
		LabID:     labID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]searchedUserResponse, len(users))
	for i, u := range users {
		resp[i] = searchedUserResponse{ID: u.ID, FirstName: u.FirstName, LastName: u.LastName, Email: u.Email}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleListLabMemberTrainingsForUser answers "which roles is this
// member trained for" -- the member-centric counterpart to
// handleListLabMemberTrainingsForRole's role-centric "who's trained for
// this role". Reuses experimentRoleResponse: the result is experiment
// roles, same shape LabSetup's Roles tab and ExperimentDetail's training-
// requirements AttachList already render.
func (s *Server) handleListLabMemberTrainingsForUser(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}

	// The outer middleware (requireLabMemberFromURL) only confirms the
	// CALLER belongs to labID -- not that {userID} does. Without this
	// check, a member of lab A could name any user id in the URL and see
	// that person's training data even if they've never belonged to lab
	// A; ListLabMemberTrainingsForUser is itself lab-scoped now too, so
	// this also keeps the two checks (route param vs. query filter)
	// from silently drifting apart.
	if _, err := s.queries.GetLabMembership(r.Context(), db.GetLabMembershipParams{UserID: userID, LabID: labID}); err != nil {
		s.writeDBError(w, err)
		return
	}

	roles, err := s.queries.ListLabMemberTrainingsForUser(r.Context(), db.ListLabMemberTrainingsForUserParams{
		UserID: userID,
		LabID:  labID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]experimentRoleResponse, len(roles))
	for i, role := range roles {
		resp[i] = experimentRoleToResponse(role)
	}
	writeJSON(w, http.StatusOK, resp)
}
