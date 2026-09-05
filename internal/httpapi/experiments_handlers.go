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
	ProtocolID         *int64   `json:"protocol_id"`
	ExperimentTypeID   *int64   `json:"experiment_type_id"`
	// CreateExperimenterRole is create-only, not persisted on the
	// experiment itself: if set, handleCreateExperiment also creates a
	// dedicated "<name> Experimenter" role and attaches it as a
	// training requirement, opt-in rather than legacy's silent always-on
	// behavior.
	CreateExperimenterRole bool `json:"create_experimenter_role"`
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
	ProtocolID         *int64    `json:"protocol_id"`
	ExperimentTypeID   *int64    `json:"experiment_type_id"`
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
		ProtocolID:         e.ProtocolID,
		ExperimentTypeID:   e.ExperimentTypeID,
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

	if !s.validateExperimentForeignKeys(w, r, labID, req) {
		return
	}

	// The experiment, its optional dedicated role, the training
	// requirement attaching that role, and both audit events all run in
	// one transaction: without this, a failure partway through (e.g. the
	// training-requirement insert) left a real experiment committed while
	// the client saw a 500, with no way to tell from the audit trail that
	// the role workflow never finished.
	var experiment db.Experiment
	var role db.ExperimentRole
	txErr := s.withTx(r.Context(), func(q db.Querier) error {
		var err error
		experiment, err = q.CreateExperiment(r.Context(), db.CreateExperimentParams{
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
			ProtocolID:         req.ProtocolID,
			ExperimentTypeID:   req.ExperimentTypeID,
		})
		if err != nil {
			return err
		}

		recorder := audit.NewRecorder(q)
		if err := recorder.Record(r.Context(), audit.Event{
			ActorUserID: currentUserID(r.Context()),
			LabID:       &experiment.LabID,
			Action:      ActionExperimentCreated,
			EntityType:  ptr("experiment"),
			EntityID:    &experiment.ID,
		}); err != nil {
			return err
		}

		if !req.CreateExperimenterRole {
			return nil
		}

		role, err = q.CreateExperimentRole(r.Context(), db.CreateExperimentRoleParams{
			LabID: experiment.LabID,
			Name:  experiment.Name + " Experimenter",
		})
		if err != nil {
			return err
		}
		if _, err := q.AddExperimentTrainingRequirement(r.Context(), db.AddExperimentTrainingRequirementParams{
			ExperimentID:     experiment.ID,
			ExperimentRoleID: role.ID,
		}); err != nil {
			return err
		}
		return recorder.Record(r.Context(), audit.Event{
			ActorUserID: currentUserID(r.Context()),
			LabID:       &experiment.LabID,
			Action:      ActionExperimentRoleCreated,
			EntityType:  ptr("experiment_role"),
			EntityID:    &role.ID,
			Metadata:    map[string]int64{"experiment_id": experiment.ID},
		})
	})
	if txErr != nil {
		s.writeDBError(w, txErr)
		return
	}

	writeJSON(w, http.StatusCreated, experimentToResponse(experiment))
}

// validateExperimentForeignKeys confirms that req's optional protocol_id
// and experiment_type_id, if set, belong to labID -- writing a 400 and
// returning ok=false otherwise. The route-level lab-membership middleware
// only checks the experiment's own lab, not whether these caller-supplied
// ids belong to it, so without this a lab member could point an
// experiment at another lab's protocol or type by guessing its id.
func (s *Server) validateExperimentForeignKeys(w http.ResponseWriter, r *http.Request, labID int64, req experimentRequest) bool {
	if req.ProtocolID != nil {
		protocol, err := s.queries.GetProtocolByID(r.Context(), *req.ProtocolID)
		if err != nil {
			s.writeDBError(w, err)
			return false
		}
		if protocol.LabID != labID {
			writeError(w, http.StatusBadRequest, "protocol not found in lab")
			return false
		}
	}
	if req.ExperimentTypeID != nil {
		experimentType, err := s.queries.GetExperimentTypeByID(r.Context(), *req.ExperimentTypeID)
		if err != nil {
			s.writeDBError(w, err)
			return false
		}
		if experimentType.LabID != labID {
			writeError(w, http.StatusBadRequest, "experiment type not found in lab")
			return false
		}
	}
	return true
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

	existing, err := s.queries.GetExperimentByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	if !s.validateExperimentForeignKeys(w, r, existing.LabID, req) {
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
		ProtocolID:         req.ProtocolID,
		ExperimentTypeID:   req.ExperimentTypeID,
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
