package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/mcdi"
)

const (
	ActionChildCreated     = "child.created"
	ActionChildUpdated     = "child.updated"
	ActionChildDeactivated = "child.deactivated"
	ActionMCDIRequested    = "child.mcdi_requested"
)

type childRequest struct {
	FirstName              string   `json:"first_name"`
	LastName               string   `json:"last_name"`
	Sex                    string   `json:"sex"`
	BirthDate              *string  `json:"birth_date"`
	DueDate                *string  `json:"due_date"`
	GestationalAgeWeeks    *float64 `json:"gestational_age_weeks"`
	BirthWeight            *float64 `json:"birth_weight"`
	Apgar1                 *int16   `json:"apgar_1"`
	Apgar2                 *int16   `json:"apgar_2"`
	Premie                 *bool    `json:"premie"`
	BirthComplications     *bool    `json:"birth_complications"`
	Twin                   *bool    `json:"twin"`
	RaceEthnicity          []string `json:"race_ethnicity"`
	Languages              []string `json:"languages"`
	RecruitmentSourceID    *int64   `json:"recruitment_source_id"`
	RecruitmentSourceOther string   `json:"recruitment_source_other"`
	Response               string   `json:"response"`
	// MCDIPercentile/MCDIDate are manual staff data entry, matching
	// legacy's last_mcdi_pct/last_mcdi_date -- nothing writes them
	// automatically. Ignored by handleCreateChild (a new child can't
	// have results yet); only handleUpdateChild uses them.
	MCDIPercentile *float64 `json:"mcdi_percentile"`
	MCDIDate       *string  `json:"mcdi_date"`
}

type childResponse struct {
	ID                     int64     `json:"id"`
	FamilyID               int64     `json:"family_id"`
	FirstName              string    `json:"first_name"`
	LastName               string    `json:"last_name"`
	Sex                    string    `json:"sex"`
	BirthDate              *string   `json:"birth_date"`
	DueDate                *string   `json:"due_date"`
	GestationalAgeWeeks    *float64  `json:"gestational_age_weeks"`
	BirthWeight            *float64  `json:"birth_weight"`
	Apgar1                 *int16    `json:"apgar_1"`
	Apgar2                 *int16    `json:"apgar_2"`
	Premie                 *bool     `json:"premie"`
	BirthComplications     *bool     `json:"birth_complications"`
	Twin                   *bool     `json:"twin"`
	RaceEthnicity          []string  `json:"race_ethnicity"`
	Languages              []string  `json:"languages"`
	RecruitmentSourceID    *int64    `json:"recruitment_source_id"`
	RecruitmentSourceOther string    `json:"recruitment_source_other"`
	Response               string    `json:"response"`
	MCDIPercentile         *float64  `json:"mcdi_percentile"`
	MCDIDate               *string   `json:"mcdi_date"`
	Deactivated            bool      `json:"deactivated"`
	InactiveReason         string    `json:"inactive_reason"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func childToResponse(c db.Child) childResponse {
	return childResponse{
		ID:                     c.ID,
		FamilyID:               c.FamilyID,
		FirstName:              c.FirstName,
		LastName:               c.LastName,
		Sex:                    c.Sex,
		BirthDate:              dateToPtr(c.BirthDate),
		DueDate:                dateToPtr(c.DueDate),
		GestationalAgeWeeks:    numericToPtr(c.GestationalAgeWeeks),
		BirthWeight:            numericToPtr(c.BirthWeight),
		Apgar1:                 c.Apgar1,
		Apgar2:                 c.Apgar2,
		Premie:                 c.Premie,
		BirthComplications:     c.BirthComplications,
		Twin:                   c.Twin,
		RaceEthnicity:          c.RaceEthnicity,
		Languages:              c.Languages,
		RecruitmentSourceID:    c.RecruitmentSourceID,
		RecruitmentSourceOther: c.RecruitmentSourceOther,
		Response:               c.Response,
		MCDIPercentile:         numericToPtr(c.McdiPercentile),
		MCDIDate:               dateToPtr(c.McdiDate),
		Deactivated:            c.DeactivatedAt.Valid,
		InactiveReason:         c.InactiveReason,
		CreatedAt:              c.CreatedAt.Time,
		UpdatedAt:              c.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateChild(w http.ResponseWriter, r *http.Request) {
	familyID, ok := idParam(w, r, "familyID")
	if !ok {
		return
	}
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return
	}

	var req childRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	birthDate, err := ptrToDate(req.BirthDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid birth_date")
		return
	}
	dueDate, err := ptrToDate(req.DueDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid due_date")
		return
	}
	gestAge, err := ptrToNumeric(req.GestationalAgeWeeks)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid gestational_age_weeks")
		return
	}
	birthWeight, err := ptrToNumeric(req.BirthWeight)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid birth_weight")
		return
	}

	child, err := s.queries.CreateChild(r.Context(), db.CreateChildParams{
		FamilyID:               familyID,
		FirstName:              req.FirstName,
		LastName:               req.LastName,
		Sex:                    req.Sex,
		BirthDate:              birthDate,
		DueDate:                dueDate,
		GestationalAgeWeeks:    gestAge,
		BirthWeight:            birthWeight,
		Apgar1:                 req.Apgar1,
		Apgar2:                 req.Apgar2,
		Premie:                 req.Premie,
		BirthComplications:     req.BirthComplications,
		Twin:                   req.Twin,
		RaceEthnicity:          nonNilSlice(req.RaceEthnicity),
		Languages:              nonNilSlice(req.Languages),
		RecruitmentSourceID:    req.RecruitmentSourceID,
		RecruitmentSourceOther: req.RecruitmentSourceOther,
		Response:               req.Response,
		CreatedByUserID:        userID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: &userID,
		Action:      ActionChildCreated,
		EntityType:  ptr("child"),
		EntityID:    &child.ID,
	})

	writeJSON(w, http.StatusCreated, childToResponse(child))
}

func (s *Server) handleGetChild(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "childID")
	if !ok {
		return
	}

	child, err := s.queries.GetChildByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, childToResponse(child))
}

func (s *Server) handleListChildrenByFamily(w http.ResponseWriter, r *http.Request) {
	familyID, ok := idParam(w, r, "familyID")
	if !ok {
		return
	}

	children, err := s.queries.ListChildrenByFamily(r.Context(), familyID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]childResponse, len(children))
	for i, c := range children {
		resp[i] = childToResponse(c)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateChild(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "childID")
	if !ok {
		return
	}

	var req childRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	birthDate, err := ptrToDate(req.BirthDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid birth_date")
		return
	}
	dueDate, err := ptrToDate(req.DueDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid due_date")
		return
	}
	gestAge, err := ptrToNumeric(req.GestationalAgeWeeks)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid gestational_age_weeks")
		return
	}
	birthWeight, err := ptrToNumeric(req.BirthWeight)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid birth_weight")
		return
	}
	mcdiPercentile, err := ptrToNumeric(req.MCDIPercentile)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid mcdi_percentile")
		return
	}
	mcdiDate, err := ptrToDate(req.MCDIDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid mcdi_date")
		return
	}

	child, err := s.queries.UpdateChild(r.Context(), db.UpdateChildParams{
		ID:                     id,
		FirstName:              req.FirstName,
		LastName:               req.LastName,
		Sex:                    req.Sex,
		BirthDate:              birthDate,
		DueDate:                dueDate,
		GestationalAgeWeeks:    gestAge,
		BirthWeight:            birthWeight,
		Apgar1:                 req.Apgar1,
		Apgar2:                 req.Apgar2,
		Premie:                 req.Premie,
		BirthComplications:     req.BirthComplications,
		Twin:                   req.Twin,
		RaceEthnicity:          nonNilSlice(req.RaceEthnicity),
		Languages:              nonNilSlice(req.Languages),
		RecruitmentSourceID:    req.RecruitmentSourceID,
		RecruitmentSourceOther: req.RecruitmentSourceOther,
		Response:               req.Response,
		McdiPercentile:         mcdiPercentile,
		McdiDate:               mcdiDate,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionChildUpdated,
		EntityType:  ptr("child"),
		EntityID:    &child.ID,
	})

	writeJSON(w, http.StatusOK, childToResponse(child))
}

type deactivateChildRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) handleDeactivateChild(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "childID")
	if !ok {
		return
	}

	var req deactivateChildRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.queries.DeactivateChild(r.Context(), db.DeactivateChildParams{ID: id, InactiveReason: req.Reason}); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionChildDeactivated,
		EntityType:  ptr("child"),
		EntityID:    &id,
		Metadata:    map[string]string{"reason": req.Reason},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSearchChildren(w http.ResponseWriter, r *http.Request) {
	minBirthDate, err := ptrToDate(queryString(r, "min_birth_date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid min_birth_date")
		return
	}
	maxBirthDate, err := ptrToDate(queryString(r, "max_birth_date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid max_birth_date")
		return
	}
	twin, ok := queryBoolPtr(w, r, "twin")
	if !ok {
		return
	}
	premie, ok := queryBoolPtr(w, r, "premie")
	if !ok {
		return
	}
	familyID, ok := queryInt64Ptr(w, r, "family_id")
	if !ok {
		return
	}
	includeDeactivated, ok := queryBool(w, r, "include_deactivated", false)
	if !ok {
		return
	}
	limit, ok := queryLimit(w, r, defaultSearchLimit, maxSearchLimit)
	if !ok {
		return
	}

	children, err := s.queries.SearchChildren(r.Context(), db.SearchChildrenParams{
		NameQuery:          queryString(r, "q"),
		MinBirthDate:       minBirthDate,
		MaxBirthDate:       maxBirthDate,
		Sex:                queryString(r, "sex"),
		Twin:               twin,
		Premie:             premie,
		Language:           queryString(r, "language"),
		FamilyID:           familyID,
		IncludeDeactivated: includeDeactivated,
		LimitCount:         limit,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]childResponse, len(children))
	for i, c := range children {
		resp[i] = childToResponse(c)
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleRequestMCDI asks daxlabbase/cdibase to email the family a
// MacArthur-Bates CDI survey link -- a direct port of legacy's manual
// "Request MCDI" button (no automation, no dedup, repeatable). Unlike
// legacy, which captured the API's response and never inspected it
// (always reporting success), a real failure here becomes a real error
// response instead of a false-positive success message.
func (s *Server) handleRequestMCDI(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "childID")
	if !ok {
		return
	}

	child, err := s.queries.GetChildByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	if !child.BirthDate.Valid {
		writeError(w, http.StatusBadRequest, "child has no birth date on file")
		return
	}

	guardians, err := s.queries.ListGuardiansByFamily(r.Context(), child.FamilyID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	var guardianEmail string
	if len(guardians) > 0 {
		guardianEmail = guardians[0].Email
	}
	if guardianEmail == "" {
		writeError(w, http.StatusBadRequest, "no guardian email on file")
		return
	}

	err = s.mcdiClient.RequestSurvey(r.Context(), mcdi.Request{
		ChildName:   child.FirstName + " " + child.LastName,
		ParentEmail: guardianEmail,
		Gender:      mcdi.GenderFor(child.Sex),
		Birthday:    child.BirthDate.Time.Format(dateLayout),
		DatabaseID:  child.ID,
	})
	if err != nil {
		s.logger.Error("request mcdi survey", "child_id", child.ID, "error", err)
		writeError(w, http.StatusBadGateway, "failed to request mcdi survey")
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionMCDIRequested,
		EntityType:  ptr("child"),
		EntityID:    &child.ID,
	})

	w.WriteHeader(http.StatusNoContent)
}
