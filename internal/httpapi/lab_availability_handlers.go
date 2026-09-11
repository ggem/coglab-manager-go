package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionLabAvailabilityGeneralCreated      = "lab_availability_general.created"
	ActionLabAvailabilityGeneralDeactivated  = "lab_availability_general.deactivated"
	ActionLabAvailabilitySpecificCreated     = "lab_availability_specific.created"
	ActionLabAvailabilitySpecificDeactivated = "lab_availability_specific.deactivated"
	// ActionLabAvailabilityManagedForMember covers the on-behalf-of paths
	// below (a coordinator/admin creating or deactivating availability for
	// someone else) -- unlike the plain self-service actions above, which
	// record no audit event at all, this is exactly the kind of privileged
	// action a lab would want a trail for.
	ActionLabAvailabilityManagedForMember = "lab_availability.managed_for_member"
)

// Availability declarations are self-service by default: a lab member
// declares their own schedule, and only they can remove it (checked in the
// deactivate handlers below, on top of the usual lab-membership check --
// being a lab member doesn't mean you can edit someone *else's* declared
// hours) -- UNLESS the caller holds this lab's "coordinator" or "admin"
// role, in which case the ForUser handlers below (and the on-behalf-of
// branch in the deactivate handlers) let them manage another member's
// schedule too.

type labAvailabilityGeneralRequest struct {
	Weekday   int16  `json:"weekday"`
	StartTime string `json:"start_time"` // "HH:MM"
	EndTime   string `json:"end_time"`
}

type labAvailabilityGeneralResponse struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	LabID     int64     `json:"lab_id"`
	Weekday   int16     `json:"weekday"`
	StartTime string    `json:"start_time"`
	EndTime   string    `json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
}

func labAvailabilityGeneralToResponse(a db.LabAvailabilityGeneral) labAvailabilityGeneralResponse {
	return labAvailabilityGeneralResponse{
		ID:        a.ID,
		UserID:    a.UserID,
		LabID:     a.LabID,
		Weekday:   a.Weekday,
		StartTime: clockTimeToString(a.StartTime),
		EndTime:   clockTimeToString(a.EndTime),
		CreatedAt: a.CreatedAt.Time,
	}
}

func (s *Server) handleCreateLabAvailabilityGeneral(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	var req labAvailabilityGeneralRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	startTime, err := stringToClockTime(req.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_time")
		return
	}
	endTime, err := stringToClockTime(req.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_time")
		return
	}

	row, err := s.queries.CreateLabAvailabilityGeneral(r.Context(), db.CreateLabAvailabilityGeneralParams{
		UserID: userID, LabID: labID, Weekday: req.Weekday,
		StartTime: startTime, EndTime: endTime,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		LabID:       &row.LabID,
		Action:      ActionLabAvailabilityGeneralCreated,
		EntityType:  ptr("lab_availability_general"),
		EntityID:    &row.ID,
	})

	writeJSON(w, http.StatusCreated, labAvailabilityGeneralToResponse(row))
}

func (s *Server) handleListLabAvailabilityGeneral(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	rows, err := s.queries.ListLabAvailabilityGeneralByUser(r.Context(), db.ListLabAvailabilityGeneralByUserParams{
		UserID: userID, LabID: labID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]labAvailabilityGeneralResponse, len(rows))
	for i, row := range rows {
		resp[i] = labAvailabilityGeneralToResponse(row)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeactivateLabAvailabilityGeneral(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "availabilityID")
	if !ok {
		return
	}
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	row, err := s.queries.GetLabAvailabilityGeneralByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	if row.UserID != userID {
		// Not this user's row: allowed only if the caller is a
		// coordinator/admin of the row's own lab -- otherwise same
		// response as "doesn't exist," matching this package's convention
		// of not distinguishing "forbidden" from "not found."
		isCoordinatorOrAdmin, err := s.queries.IsLabCoordinatorOrAdmin(r.Context(), db.IsLabCoordinatorOrAdminParams{
			UserID: userID, LabID: row.LabID,
		})
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !isCoordinatorOrAdmin {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
	}

	if err := s.queries.DeactivateLabAvailabilityGeneral(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	action := ActionLabAvailabilityGeneralDeactivated
	var metadata map[string]any
	if row.UserID != userID {
		action = ActionLabAvailabilityManagedForMember
		metadata = map[string]any{"user_id": row.UserID}
	}
	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		LabID:       &row.LabID,
		Action:      action,
		EntityType:  ptr("lab_availability_general"),
		EntityID:    &id,
		Metadata:    metadata,
	})

	w.WriteHeader(http.StatusNoContent)
}

type labAvailabilitySpecificRequest struct {
	Date      string `json:"date"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type labAvailabilitySpecificResponse struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	LabID     int64     `json:"lab_id"`
	Date      string    `json:"date"`
	StartTime string    `json:"start_time"`
	EndTime   string    `json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
}

func labAvailabilitySpecificToResponse(a db.LabAvailabilitySpecific) labAvailabilitySpecificResponse {
	date := ""
	if a.Date.Valid {
		date = a.Date.Time.Format(dateLayout)
	}
	return labAvailabilitySpecificResponse{
		ID:        a.ID,
		UserID:    a.UserID,
		LabID:     a.LabID,
		Date:      date,
		StartTime: clockTimeToString(a.StartTime),
		EndTime:   clockTimeToString(a.EndTime),
		CreatedAt: a.CreatedAt.Time,
	}
}

func (s *Server) handleCreateLabAvailabilitySpecific(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	var req labAvailabilitySpecificRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	date, err := ptrToDate(&req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	startTime, err := stringToClockTime(req.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_time")
		return
	}
	endTime, err := stringToClockTime(req.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_time")
		return
	}

	row, err := s.queries.CreateLabAvailabilitySpecific(r.Context(), db.CreateLabAvailabilitySpecificParams{
		UserID: userID, LabID: labID, Date: date,
		StartTime: startTime, EndTime: endTime,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		LabID:       &row.LabID,
		Action:      ActionLabAvailabilitySpecificCreated,
		EntityType:  ptr("lab_availability_specific"),
		EntityID:    &row.ID,
	})

	writeJSON(w, http.StatusCreated, labAvailabilitySpecificToResponse(row))
}

func (s *Server) handleListLabAvailabilitySpecific(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	rows, err := s.queries.ListLabAvailabilitySpecificByUser(r.Context(), db.ListLabAvailabilitySpecificByUserParams{
		UserID: userID, LabID: labID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]labAvailabilitySpecificResponse, len(rows))
	for i, row := range rows {
		resp[i] = labAvailabilitySpecificToResponse(row)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeactivateLabAvailabilitySpecific(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "availabilityID")
	if !ok {
		return
	}
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	row, err := s.queries.GetLabAvailabilitySpecificByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	if row.UserID != userID {
		// Not this user's row: allowed only if the caller is a
		// coordinator/admin of the row's own lab -- otherwise same
		// response as "doesn't exist," matching this package's convention
		// of not distinguishing "forbidden" from "not found."
		isCoordinatorOrAdmin, err := s.queries.IsLabCoordinatorOrAdmin(r.Context(), db.IsLabCoordinatorOrAdminParams{
			UserID: userID, LabID: row.LabID,
		})
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !isCoordinatorOrAdmin {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
	}

	if err := s.queries.DeactivateLabAvailabilitySpecific(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	action := ActionLabAvailabilitySpecificDeactivated
	var metadata map[string]any
	if row.UserID != userID {
		action = ActionLabAvailabilityManagedForMember
		metadata = map[string]any{"user_id": row.UserID}
	}
	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		LabID:       &row.LabID,
		Action:      action,
		EntityType:  ptr("lab_availability_specific"),
		EntityID:    &id,
		Metadata:    metadata,
	})

	w.WriteHeader(http.StatusNoContent)
}

// The four handlers below are the coordinator/admin counterparts to the
// self-service list/create handlers above: same queries, but the target
// user comes from the URL (nested under a specific member's
// /memberships/{userID}/...) instead of the caller's own session. Gated by
// requireLabCoordinatorOrAdminFromURL at the route level, so by the time
// these run the caller is already known to hold this lab's "coordinator"
// or "admin" role. Deactivation needs no ForUser counterpart: it's already
// keyed by row ID, not by which user's list it came from, so the existing
// deactivate handlers' on-behalf-of branch (above) covers it.

func (s *Server) handleListLabAvailabilityGeneralForUser(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}

	// The outer middleware (requireLabCoordinatorOrAdminFromURL) only
	// confirms the CALLER is a coordinator/admin of labID -- not that
	// {userID} actually belongs to labID at all. Without this check, a
	// coordinator of lab 9 could name any user id in the URL and manage
	// availability for someone who's never been a member of lab 9,
	// contradicting "manage other members of *their* lab" -- same
	// reasoning as handleListLabMemberTrainingsForUser's own membership
	// check.
	if _, err := s.queries.GetLabMembership(r.Context(), db.GetLabMembershipParams{UserID: userID, LabID: labID}); err != nil {
		s.writeDBError(w, err)
		return
	}

	rows, err := s.queries.ListLabAvailabilityGeneralByUser(r.Context(), db.ListLabAvailabilityGeneralByUserParams{
		UserID: userID, LabID: labID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]labAvailabilityGeneralResponse, len(rows))
	for i, row := range rows {
		resp[i] = labAvailabilityGeneralToResponse(row)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateLabAvailabilityGeneralForUser(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	targetUserID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	callerID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	// Same membership check as the list handler above -- without it a
	// coordinator of labID could create availability for a user who
	// isn't actually a member of labID at all.
	if _, err := s.queries.GetLabMembership(r.Context(), db.GetLabMembershipParams{UserID: targetUserID, LabID: labID}); err != nil {
		s.writeDBError(w, err)
		return
	}

	var req labAvailabilityGeneralRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	startTime, err := stringToClockTime(req.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_time")
		return
	}
	endTime, err := stringToClockTime(req.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_time")
		return
	}

	row, err := s.queries.CreateLabAvailabilityGeneral(r.Context(), db.CreateLabAvailabilityGeneralParams{
		UserID: targetUserID, LabID: labID, Weekday: req.Weekday,
		StartTime: startTime, EndTime: endTime,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &callerID,
		LabID:       &row.LabID,
		Action:      ActionLabAvailabilityManagedForMember,
		EntityType:  ptr("lab_availability_general"),
		EntityID:    &row.ID,
		Metadata:    map[string]any{"user_id": targetUserID},
	})

	writeJSON(w, http.StatusCreated, labAvailabilityGeneralToResponse(row))
}

func (s *Server) handleListLabAvailabilitySpecificForUser(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}

	// See handleListLabAvailabilityGeneralForUser's comment: confirms
	// {userID} actually belongs to labID, which the outer middleware
	// doesn't check.
	if _, err := s.queries.GetLabMembership(r.Context(), db.GetLabMembershipParams{UserID: userID, LabID: labID}); err != nil {
		s.writeDBError(w, err)
		return
	}

	rows, err := s.queries.ListLabAvailabilitySpecificByUser(r.Context(), db.ListLabAvailabilitySpecificByUserParams{
		UserID: userID, LabID: labID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]labAvailabilitySpecificResponse, len(rows))
	for i, row := range rows {
		resp[i] = labAvailabilitySpecificToResponse(row)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateLabAvailabilitySpecificForUser(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	targetUserID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	callerID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	// See handleCreateLabAvailabilityGeneralForUser's comment.
	if _, err := s.queries.GetLabMembership(r.Context(), db.GetLabMembershipParams{UserID: targetUserID, LabID: labID}); err != nil {
		s.writeDBError(w, err)
		return
	}

	var req labAvailabilitySpecificRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	date, err := ptrToDate(&req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	startTime, err := stringToClockTime(req.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_time")
		return
	}
	endTime, err := stringToClockTime(req.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_time")
		return
	}

	row, err := s.queries.CreateLabAvailabilitySpecific(r.Context(), db.CreateLabAvailabilitySpecificParams{
		UserID: targetUserID, LabID: labID, Date: date,
		StartTime: startTime, EndTime: endTime,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &callerID,
		LabID:       &row.LabID,
		Action:      ActionLabAvailabilityManagedForMember,
		EntityType:  ptr("lab_availability_specific"),
		EntityID:    &row.ID,
		Metadata:    map[string]any{"user_id": targetUserID},
	})

	writeJSON(w, http.StatusCreated, labAvailabilitySpecificToResponse(row))
}
