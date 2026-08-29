package httpapi

import (
	"math/rand/v2"
	"net/http"
	"slices"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

// maxHoldCount caps how many children one hold-children call can hold at
// once -- there's no legacy precedent for a limit, but nothing about
// selecting candidates needs an unbounded batch, and it keeps one request
// from creating an unreasonable number of appointment rows.
const maxHoldCount = 100

type holdChildrenRequest struct {
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	Count     int     `json:"count"`
	Sort      string  `json:"sort"` // "oldest" (default) or "random"
	Sex       *string `json:"sex"`
}

// handleHoldChildrenForExperiment finds up to Count eligible, not-
// currently-held children for the experiment within [StartDate, EndDate]
// and immediately holds every one it picks by creating a to_be_scheduled
// appointment for each -- matching legacy's select-children ->
// hold-children being one atomic action, not a preview-then-commit flow.
//
// A candidate that another concurrent hold-children call grabs first
// (a unique violation on appointments_one_active_hold_per_child) is
// skipped, not treated as a request failure: this is the same
// best-effort selection legacy has, just race-safe now instead of
// racy. The response can hold fewer than Count children if the eligible
// pool is smaller or a race consumed a candidate -- that's expected.
func (s *Server) handleHoldChildrenForExperiment(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	var req holdChildrenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Count <= 0 || req.Count > maxHoldCount {
		writeError(w, http.StatusBadRequest, "count must be between 1 and 100")
		return
	}
	if req.Sort == "" {
		req.Sort = "oldest"
	}
	if req.Sort != "oldest" && req.Sort != "random" {
		writeError(w, http.StatusBadRequest, "sort must be \"oldest\" or \"random\"")
		return
	}

	start, err := ptrToDate(&req.StartDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_date")
		return
	}
	end, err := ptrToDate(&req.EndDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_date")
		return
	}
	if end.Time.Before(start.Time) {
		start, end = end, start
	}

	candidates, err := s.queries.ListEligibleChildrenForExperiment(r.Context(), db.ListEligibleChildrenForExperimentParams{
		ExperimentID: experimentID,
		WindowStart:  start,
		WindowEnd:    end,
		Sex:          req.Sex,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	if req.Sort == "random" {
		rand.Shuffle(len(candidates), func(i, j int) {
			candidates[i], candidates[j] = candidates[j], candidates[i]
		})
	} else {
		slices.SortFunc(candidates, func(a, b db.Child) int {
			return a.BirthDate.Time.Compare(b.BirthDate.Time)
		})
	}
	if len(candidates) > req.Count {
		candidates = candidates[:req.Count]
	}

	held := make([]appointmentResponse, 0, len(candidates))
	for _, child := range candidates {
		appointment, err := s.queries.CreateAppointment(r.Context(), db.CreateAppointmentParams{
			ExperimentID:  experimentID,
			ChildID:       child.ID,
			Session:       1,
			SiblingComing: "unknown",
		})
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			s.writeDBError(w, err)
			return
		}

		s.recordAuditEvent(r, audit.Event{
			ActorUserID: currentUserID(r.Context()),
			Action:      ActionAppointmentCreated,
			EntityType:  ptr("appointment"),
			EntityID:    &appointment.ID,
		})
		held = append(held, appointmentToResponse(appointment))
	}

	writeJSON(w, http.StatusOK, held)
}
