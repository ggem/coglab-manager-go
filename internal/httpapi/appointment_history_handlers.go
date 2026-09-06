package httpapi

import "net/http"

// A child's "Previous Studies" (own + siblings') appointment history --
// shown on the hold-selection screen alongside the child's contact
// details and call log, matching legacy's child-id->experiments /
// child-id->sibling-experiments. Deliberately excludes still-unscheduled
// (to_be_scheduled) appointments at the query level: an unscheduled hold
// isn't a "study," it's the very appointment this screen is working
// through.

type appointmentHistoryEntry struct {
	AppointmentID  int64   `json:"appointment_id"`
	ExperimentName string  `json:"experiment_name"`
	Status         string  `json:"status"`
	ScheduleDate   *string `json:"schedule_date"`
}

type siblingAppointmentHistoryEntry struct {
	AppointmentID  int64   `json:"appointment_id"`
	ExperimentName string  `json:"experiment_name"`
	Status         string  `json:"status"`
	ScheduleDate   *string `json:"schedule_date"`
	ChildFirstName string  `json:"child_first_name"`
	ChildLastName  string  `json:"child_last_name"`
}

type childAppointmentHistoryResponse struct {
	Own      []appointmentHistoryEntry        `json:"own"`
	Siblings []siblingAppointmentHistoryEntry `json:"siblings"`
}

// handleGetChildAppointmentHistory returns both halves of a child's
// appointment history in one response -- the frontend renders them as a
// single "Previous Studies" section, so one round trip beats two.
func (s *Server) handleGetChildAppointmentHistory(w http.ResponseWriter, r *http.Request) {
	childID, ok := idParam(w, r, "childID")
	if !ok {
		return
	}

	own, err := s.queries.ListAppointmentsByChild(r.Context(), childID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	siblings, err := s.queries.ListAppointmentsBySiblings(r.Context(), childID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := childAppointmentHistoryResponse{
		Own:      make([]appointmentHistoryEntry, len(own)),
		Siblings: make([]siblingAppointmentHistoryEntry, len(siblings)),
	}
	for i, a := range own {
		resp.Own[i] = appointmentHistoryEntry{
			AppointmentID:  a.ID,
			ExperimentName: a.ExperimentName,
			Status:         a.Status,
			ScheduleDate:   dateToPtr(a.ScheduleDate),
		}
	}
	for i, a := range siblings {
		resp.Siblings[i] = siblingAppointmentHistoryEntry{
			AppointmentID:  a.ID,
			ExperimentName: a.ExperimentName,
			Status:         a.Status,
			ScheduleDate:   dateToPtr(a.ScheduleDate),
			ChildFirstName: a.ChildFirstName,
			ChildLastName:  a.ChildLastName,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
