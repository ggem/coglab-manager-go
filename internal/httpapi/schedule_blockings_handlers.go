package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionScheduleBlockingCreated     = "schedule_blocking.created"
	ActionScheduleBlockingDeactivated = "schedule_blocking.deactivated"
)

type scheduleBlockingRequest struct {
	Date      string `json:"date"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Reason    string `json:"reason"`
}

type scheduleBlockingResponse struct {
	ID        int64     `json:"id"`
	LabID     int64     `json:"lab_id"`
	Date      string    `json:"date"`
	StartTime string    `json:"start_time"`
	EndTime   string    `json:"end_time"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func scheduleBlockingToResponse(b db.ScheduleBlocking) scheduleBlockingResponse {
	date := ""
	if b.Date.Valid {
		date = b.Date.Time.Format(dateLayout)
	}
	return scheduleBlockingResponse{
		ID:        b.ID,
		LabID:     b.LabID,
		Date:      date,
		StartTime: clockTimeToString(b.StartTime),
		EndTime:   clockTimeToString(b.EndTime),
		Reason:    b.Reason,
		CreatedAt: b.CreatedAt.Time,
	}
}

func (s *Server) handleCreateScheduleBlocking(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req scheduleBlockingRequest
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

	blocking, err := s.queries.CreateScheduleBlocking(r.Context(), db.CreateScheduleBlockingParams{
		LabID: labID, Date: date, StartTime: startTime, EndTime: endTime, Reason: req.Reason,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &blocking.LabID,
		Action:      ActionScheduleBlockingCreated,
		EntityType:  ptr("schedule_blocking"),
		EntityID:    &blocking.ID,
	})

	writeJSON(w, http.StatusCreated, scheduleBlockingToResponse(blocking))
}

func (s *Server) handleListScheduleBlockingsByLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	blockings, err := s.queries.ListScheduleBlockingsByLab(r.Context(), labID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]scheduleBlockingResponse, len(blockings))
	for i, b := range blockings {
		resp[i] = scheduleBlockingToResponse(b)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeactivateScheduleBlocking(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "blockingID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateScheduleBlocking(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionScheduleBlockingDeactivated,
		EntityType:  ptr("schedule_blocking"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}
