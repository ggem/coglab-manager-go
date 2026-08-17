package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionChildCreated     = "child.created"
	ActionChildUpdated     = "child.updated"
	ActionChildDeactivated = "child.deactivated"
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
		RaceEthnicity:          req.RaceEthnicity,
		Languages:              req.Languages,
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
		RaceEthnicity:          req.RaceEthnicity,
		Languages:              req.Languages,
		RecruitmentSourceID:    req.RecruitmentSourceID,
		RecruitmentSourceOther: req.RecruitmentSourceOther,
		Response:               req.Response,
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
