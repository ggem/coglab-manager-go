package httpapi

import (
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionFamilyCreated = "family.created"
	ActionFamilyUpdated = "family.updated"
)

type familyRequest struct {
	Address                string  `json:"address"`
	City                   string  `json:"city"`
	State                  string  `json:"state"`
	Zip                    string  `json:"zip"`
	PreferredContactMethod *string `json:"preferred_contact_method"`
}

type familyResponse struct {
	ID                     int64     `json:"id"`
	Address                string    `json:"address"`
	City                   string    `json:"city"`
	State                  string    `json:"state"`
	Zip                    string    `json:"zip"`
	PreferredContactMethod *string   `json:"preferred_contact_method"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func familyToResponse(f db.Family) familyResponse {
	return familyResponse{
		ID:                     f.ID,
		Address:                f.Address,
		City:                   f.City,
		State:                  f.State,
		Zip:                    f.Zip,
		PreferredContactMethod: f.PreferredContactMethod,
		CreatedAt:              f.CreatedAt.Time,
		UpdatedAt:              f.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateFamily(w http.ResponseWriter, r *http.Request) {
	var req familyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	family, err := s.queries.CreateFamily(r.Context(), db.CreateFamilyParams{
		Address:                req.Address,
		City:                   req.City,
		State:                  req.State,
		Zip:                    req.Zip,
		PreferredContactMethod: req.PreferredContactMethod,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionFamilyCreated,
		EntityType:  ptr("family"),
		EntityID:    &family.ID,
	})

	writeJSON(w, http.StatusCreated, familyToResponse(family))
}

func (s *Server) handleGetFamily(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "familyID")
	if !ok {
		return
	}

	family, err := s.queries.GetFamilyByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, familyToResponse(family))
}

func (s *Server) handleUpdateFamily(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "familyID")
	if !ok {
		return
	}

	var req familyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	family, err := s.queries.UpdateFamily(r.Context(), db.UpdateFamilyParams{
		ID:                     id,
		Address:                req.Address,
		City:                   req.City,
		State:                  req.State,
		Zip:                    req.Zip,
		PreferredContactMethod: req.PreferredContactMethod,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionFamilyUpdated,
		EntityType:  ptr("family"),
		EntityID:    &family.ID,
	})

	writeJSON(w, http.StatusOK, familyToResponse(family))
}

func (s *Server) handleSearchFamilies(w http.ResponseWriter, r *http.Request) {
	limit, ok := queryLimit(w, r, defaultSearchLimit, maxSearchLimit)
	if !ok {
		return
	}

	families, err := s.queries.SearchFamilies(r.Context(), db.SearchFamiliesParams{
		NameQuery:   queryString(r, "q"),
		Email:       queryString(r, "email"),
		PhoneNumber: queryString(r, "phone_number"),
		Address:     queryString(r, "address"),
		City:        queryString(r, "city"),
		Zip:         queryString(r, "zip"),
		LimitCount:  limit,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]familyResponse, len(families))
	for i, f := range families {
		resp[i] = familyToResponse(f)
	}
	writeJSON(w, http.StatusOK, resp)
}
