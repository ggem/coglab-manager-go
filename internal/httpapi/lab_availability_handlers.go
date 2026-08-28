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
)

// Availability declarations are self-service: a lab member declares their
// own schedule, and only they can remove it (checked in the deactivate
// handlers below, on top of the usual lab-membership check -- being a lab
// member doesn't mean you can edit someone *else's* declared hours).
// There's no role-based "a coordinator manages everyone's schedule"
// capability yet; that's part of the deferred role-based-permissions work.

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
		// Not this user's row: same response as "doesn't exist," matching
		// this package's convention of not distinguishing "forbidden"
		// from "not found."
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if err := s.queries.DeactivateLabAvailabilityGeneral(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		LabID:       &row.LabID,
		Action:      ActionLabAvailabilityGeneralDeactivated,
		EntityType:  ptr("lab_availability_general"),
		EntityID:    &id,
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
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if err := s.queries.DeactivateLabAvailabilitySpecific(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		LabID:       &row.LabID,
		Action:      ActionLabAvailabilitySpecificDeactivated,
		EntityType:  ptr("lab_availability_specific"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}
