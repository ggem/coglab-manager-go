package httpapi

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionExperimentCreated     = "experiment.created"
	ActionExperimentUpdated     = "experiment.updated"
	ActionExperimentDeactivated = "experiment.deactivated"
)

type experimentRequest struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Sessions           int16    `json:"sessions"`
	AgeRangeMinMonths  *float64 `json:"age_range_min_months"`
	AgeRangeMaxMonths  *float64 `json:"age_range_max_months"`
	StartDate          *string  `json:"start_date"`
	EndDate            *string  `json:"end_date"`
	Status             string   `json:"status"`
	DurationMinutes    int16    `json:"duration_minutes"`
	FilterPremies      bool     `json:"filter_premies"`
	FilterMinLanguages int16    `json:"filter_min_languages"`
	FilterLanguages    []string `json:"filter_languages"`
}

type experimentResponse struct {
	ID                 int64     `json:"id"`
	LabID              int64     `json:"lab_id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Sessions           int16     `json:"sessions"`
	AgeRangeMinMonths  *float64  `json:"age_range_min_months"`
	AgeRangeMaxMonths  *float64  `json:"age_range_max_months"`
	StartDate          *string   `json:"start_date"`
	EndDate            *string   `json:"end_date"`
	Status             string    `json:"status"`
	DurationMinutes    int16     `json:"duration_minutes"`
	FilterPremies      bool      `json:"filter_premies"`
	FilterMinLanguages int16     `json:"filter_min_languages"`
	FilterLanguages    []string  `json:"filter_languages"`
	Deactivated        bool      `json:"deactivated"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func experimentToResponse(e db.Experiment) experimentResponse {
	return experimentResponse{
		ID:                 e.ID,
		LabID:              e.LabID,
		Name:               e.Name,
		Description:        e.Description,
		Sessions:           e.Sessions,
		AgeRangeMinMonths:  numericToPtr(e.AgeRangeMinMonths),
		AgeRangeMaxMonths:  numericToPtr(e.AgeRangeMaxMonths),
		StartDate:          dateToPtr(e.StartDate),
		EndDate:            dateToPtr(e.EndDate),
		Status:             e.Status,
		DurationMinutes:    e.DurationMinutes,
		FilterPremies:      e.FilterPremies,
		FilterMinLanguages: e.FilterMinLanguages,
		FilterLanguages:    e.FilterLanguages,
		Deactivated:        e.DeactivatedAt.Valid,
		CreatedAt:          e.CreatedAt.Time,
		UpdatedAt:          e.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateExperiment(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req experimentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ageMin, ageMax, startDate, endDate, ok := decodeExperimentFields(w, req)
	if !ok {
		return
	}

	experiment, err := s.queries.CreateExperiment(r.Context(), db.CreateExperimentParams{
		LabID:              labID,
		Name:               req.Name,
		Description:        req.Description,
		Sessions:           req.Sessions,
		AgeRangeMinMonths:  ageMin,
		AgeRangeMaxMonths:  ageMax,
		StartDate:          startDate,
		EndDate:            endDate,
		Status:             req.Status,
		DurationMinutes:    req.DurationMinutes,
		FilterPremies:      req.FilterPremies,
		FilterMinLanguages: req.FilterMinLanguages,
		FilterLanguages:    nonNilSlice(req.FilterLanguages),
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &experiment.LabID,
		Action:      ActionExperimentCreated,
		EntityType:  ptr("experiment"),
		EntityID:    &experiment.ID,
	})

	writeJSON(w, http.StatusCreated, experimentToResponse(experiment))
}

func (s *Server) handleGetExperiment(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	experiment, err := s.queries.GetExperimentByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, experimentToResponse(experiment))
}

func (s *Server) handleListExperimentsByLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	experiments, err := s.queries.ListExperimentsByLab(r.Context(), labID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]experimentResponse, len(experiments))
	for i, e := range experiments {
		resp[i] = experimentToResponse(e)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateExperiment(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	var req experimentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ageMin, ageMax, startDate, endDate, ok := decodeExperimentFields(w, req)
	if !ok {
		return
	}

	experiment, err := s.queries.UpdateExperiment(r.Context(), db.UpdateExperimentParams{
		ID:                 id,
		Name:               req.Name,
		Description:        req.Description,
		Sessions:           req.Sessions,
		AgeRangeMinMonths:  ageMin,
		AgeRangeMaxMonths:  ageMax,
		StartDate:          startDate,
		EndDate:            endDate,
		Status:             req.Status,
		DurationMinutes:    req.DurationMinutes,
		FilterPremies:      req.FilterPremies,
		FilterMinLanguages: req.FilterMinLanguages,
		FilterLanguages:    nonNilSlice(req.FilterLanguages),
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &experiment.LabID,
		Action:      ActionExperimentUpdated,
		EntityType:  ptr("experiment"),
		EntityID:    &experiment.ID,
	})

	writeJSON(w, http.StatusOK, experimentToResponse(experiment))
}

func (s *Server) handleDeactivateExperiment(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateExperiment(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionExperimentDeactivated,
		EntityType:  ptr("experiment"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}

// decodeExperimentFields converts the pointer-typed date/numeric fields
// shared by create and update, writing a 400 and returning ok=false on the
// first invalid one.
func decodeExperimentFields(w http.ResponseWriter, req experimentRequest) (ageMin, ageMax pgtype.Numeric, startDate, endDate pgtype.Date, ok bool) {
	ageMin, err := ptrToNumeric(req.AgeRangeMinMonths)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid age_range_min_months")
		return pgtype.Numeric{}, pgtype.Numeric{}, pgtype.Date{}, pgtype.Date{}, false
	}
	ageMax, err = ptrToNumeric(req.AgeRangeMaxMonths)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid age_range_max_months")
		return pgtype.Numeric{}, pgtype.Numeric{}, pgtype.Date{}, pgtype.Date{}, false
	}
	startDate, err = ptrToDate(req.StartDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_date")
		return pgtype.Numeric{}, pgtype.Numeric{}, pgtype.Date{}, pgtype.Date{}, false
	}
	endDate, err = ptrToDate(req.EndDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_date")
		return pgtype.Numeric{}, pgtype.Numeric{}, pgtype.Date{}, pgtype.Date{}, false
	}
	return ageMin, ageMax, startDate, endDate, true
}
