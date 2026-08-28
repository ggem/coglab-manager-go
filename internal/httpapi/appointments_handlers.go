package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/scheduling"
)

const (
	ActionAppointmentCreated   = "appointment.created"
	ActionAppointmentScheduled = "appointment.scheduled"
)

// maxSearchDays caps how many days an availability search covers in one
// request, matching the legacy engine's cap -- a search's cost grows with
// the number of days, and nothing about scheduling one appointment needs
// an unbounded range.
const maxSearchDays = 28

type appointmentRequest struct {
	ChildID           int64    `json:"child_id"`
	Session           int16    `json:"session"`
	AgeRangeMinMonths *float64 `json:"age_range_min_months"`
	AgeRangeMaxMonths *float64 `json:"age_range_max_months"`
	SiblingComing     string   `json:"sibling_coming"`
}

type appointmentResponse struct {
	ID                int64     `json:"id"`
	ExperimentID      int64     `json:"experiment_id"`
	ChildID           int64     `json:"child_id"`
	Session           int16     `json:"session"`
	AgeRangeMinMonths *float64  `json:"age_range_min_months"`
	AgeRangeMaxMonths *float64  `json:"age_range_max_months"`
	SiblingComing     string    `json:"sibling_coming"`
	ScheduleDate      *string   `json:"schedule_date"`
	ScheduleTimeStart *string   `json:"schedule_time_start"`
	ScheduleTimeEnd   *string   `json:"schedule_time_end"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

func appointmentToResponse(a db.Appointment) appointmentResponse {
	resp := appointmentResponse{
		ID:                a.ID,
		ExperimentID:      a.ExperimentID,
		ChildID:           a.ChildID,
		Session:           a.Session,
		AgeRangeMinMonths: numericToPtr(a.AgeRangeMinMonths),
		AgeRangeMaxMonths: numericToPtr(a.AgeRangeMaxMonths),
		SiblingComing:     a.SiblingComing,
		ScheduleDate:      dateToPtr(a.ScheduleDate),
		Status:            a.Status,
		CreatedAt:         a.CreatedAt.Time,
	}
	if a.ScheduleTimeStart.Valid {
		resp.ScheduleTimeStart = ptr(clockTimeToString(a.ScheduleTimeStart))
	}
	if a.ScheduleTimeEnd.Valid {
		resp.ScheduleTimeEnd = ptr(clockTimeToString(a.ScheduleTimeEnd))
	}
	return resp
}

func (s *Server) handleCreateAppointment(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	var req appointmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Session == 0 {
		req.Session = 1
	}
	if req.SiblingComing == "" {
		req.SiblingComing = "unknown"
	}
	ageMin, err := ptrToNumeric(req.AgeRangeMinMonths)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid age_range_min_months")
		return
	}
	ageMax, err := ptrToNumeric(req.AgeRangeMaxMonths)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid age_range_max_months")
		return
	}

	appointment, err := s.queries.CreateAppointment(r.Context(), db.CreateAppointmentParams{
		ExperimentID:      experimentID,
		ChildID:           req.ChildID,
		Session:           req.Session,
		AgeRangeMinMonths: ageMin,
		AgeRangeMaxMonths: ageMax,
		SiblingComing:     req.SiblingComing,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionAppointmentCreated,
		EntityType:  ptr("appointment"),
		EntityID:    &appointment.ID,
	})

	writeJSON(w, http.StatusCreated, appointmentToResponse(appointment))
}

type candidateSlotResponse struct {
	Date       string          `json:"date"`
	StartTime  string          `json:"start_time"`
	Assignment map[int64]int64 `json:"assignment"` // role ID -> user ID
	GreeterID  int64           `json:"greeter_id"`
	HasSitter  bool            `json:"has_sitter"`
}

func candidateSlotToResponse(c scheduling.CandidateSlot) candidateSlotResponse {
	return candidateSlotResponse{
		Date:       c.Date.Format(dateLayout),
		StartTime:  durationToClock(c.StartTime),
		Assignment: c.Assignment,
		GreeterID:  c.GreeterID,
		HasSitter:  c.HasSitter,
	}
}

func (s *Server) handleSearchAppointmentAvailability(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := idParam(w, r, "appointmentID")
	if !ok {
		return
	}

	startDateStr := queryString(r, "start_date")
	endDateStr := queryString(r, "end_date")
	if startDateStr == nil || endDateStr == nil {
		writeError(w, http.StatusBadRequest, "start_date and end_date are required")
		return
	}
	startDate, err := time.Parse(dateLayout, *startDateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_date")
		return
	}
	endDate, err := time.Parse(dateLayout, *endDateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_date")
		return
	}
	if endDate.Before(startDate) {
		startDate, endDate = endDate, startDate
	}

	appointment, err := s.queries.GetAppointmentByID(r.Context(), appointmentID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	experiment, err := s.queries.GetExperimentByID(r.Context(), appointment.ExperimentID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	search, err := s.buildAvailabilitySearch(r.Context(), experiment, appointment, startDate, endDate)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	results := scheduling.SearchAvailability(
		search.days, search.roles, search.sitterRole, search.sitterRequirement,
		time.Duration(experiment.DurationMinutes)*time.Minute,
	)

	resp := make([]candidateSlotResponse, len(results))
	for i, c := range results {
		resp[i] = candidateSlotToResponse(c)
	}
	writeJSON(w, http.StatusOK, resp)
}

type scheduleAppointmentRequest struct {
	Date      string `json:"date"`
	StartTime string `json:"start_time"`
}

// handleScheduleAppointment commits a chosen date/time: it re-derives a
// fresh assignment for exactly that slot (a single-day, single-time
// search) rather than trusting whatever the client saw in an earlier
// search response, since time passes between a search and a commit --
// matching the legacy engine's "Schedule This" step re-checking rather
// than reusing a stale result. If a client wants a specific staff
// combination rather than whichever one this finds, that's the deferred
// manual-reassignment work.
func (s *Server) handleScheduleAppointment(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := idParam(w, r, "appointmentID")
	if !ok {
		return
	}

	var req scheduleAppointmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	date, err := time.Parse(dateLayout, req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	startTime, err := stringToClockTime(req.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_time")
		return
	}

	appointment, err := s.queries.GetAppointmentByID(r.Context(), appointmentID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	experiment, err := s.queries.GetExperimentByID(r.Context(), appointment.ExperimentID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	search, err := s.buildAvailabilitySearch(r.Context(), experiment, appointment, date, date)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	duration := time.Duration(experiment.DurationMinutes) * time.Minute
	results := scheduling.SearchAvailability(search.days, search.roles, search.sitterRole, search.sitterRequirement, duration)

	requestedStart := pgTimeToDuration(startTime)
	var chosen *scheduling.CandidateSlot
	for i := range results {
		if results[i].StartTime == requestedStart {
			chosen = &results[i]
			break
		}
	}
	if chosen == nil {
		writeError(w, http.StatusConflict, "that slot is no longer available")
		return
	}

	scheduleDate, err := ptrToDate(&req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	endTime, err := stringToClockTime(durationToClock(requestedStart + duration))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	scheduled, err := s.queries.ScheduleAppointment(r.Context(), db.ScheduleAppointmentParams{
		ID:                appointmentID,
		ScheduleDate:      scheduleDate,
		ScheduleTimeStart: startTime,
		ScheduleTimeEnd:   endTime,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	for roleID, userID := range chosen.Assignment {
		if _, err := s.queries.CreateAppointmentExperimenter(r.Context(), db.CreateAppointmentExperimenterParams{
			AppointmentID:    appointmentID,
			UserID:           userID,
			ExperimentRoleID: roleID,
			IsGreeter:        userID == chosen.GreeterID,
		}); err != nil {
			s.writeDBError(w, err)
			return
		}
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionAppointmentScheduled,
		EntityType:  ptr("appointment"),
		EntityID:    &appointmentID,
	})

	writeJSON(w, http.StatusOK, appointmentToResponse(scheduled))
}
