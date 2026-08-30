package httpapi

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"time"

	"github.com/ggem/coglab-manager-go/internal/audit"
	"github.com/ggem/coglab-manager-go/internal/db"
)

const (
	ActionNewsletterCreated     = "newsletter.created"
	ActionNewsletterDeactivated = "newsletter.deactivated"
	ActionNewsletterMarkedSent  = "newsletter.marked_sent"
)

type newsletterRequest struct {
	Name string `json:"name"`
}

type newsletterResponse struct {
	ID          int64     `json:"id"`
	LabID       int64     `json:"lab_id"`
	Name        string    `json:"name"`
	Deactivated bool      `json:"deactivated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newsletterToResponse(n db.Newsletter) newsletterResponse {
	return newsletterResponse{
		ID:          n.ID,
		LabID:       n.LabID,
		Name:        n.Name,
		Deactivated: n.DeactivatedAt.Valid,
		CreatedAt:   n.CreatedAt.Time,
		UpdatedAt:   n.UpdatedAt.Time,
	}
}

func (s *Server) handleCreateNewsletter(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	var req newsletterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	newsletter, err := s.queries.CreateNewsletter(r.Context(), db.CreateNewsletterParams{
		LabID: labID,
		Name:  req.Name,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &newsletter.LabID,
		Action:      ActionNewsletterCreated,
		EntityType:  ptr("newsletter"),
		EntityID:    &newsletter.ID,
	})

	writeJSON(w, http.StatusCreated, newsletterToResponse(newsletter))
}

func (s *Server) handleGetNewsletter(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "newsletterID")
	if !ok {
		return
	}

	newsletter, err := s.queries.GetNewsletterByID(r.Context(), id)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, newsletterToResponse(newsletter))
}

func (s *Server) handleListNewslettersByLab(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}

	newsletters, err := s.queries.ListNewslettersByLab(r.Context(), labID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]newsletterResponse, len(newsletters))
	for i, n := range newsletters {
		resp[i] = newsletterToResponse(n)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeactivateNewsletter(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "newsletterID")
	if !ok {
		return
	}

	if err := s.queries.DeactivateNewsletter(r.Context(), id); err != nil {
		s.writeDBError(w, err)
		return
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		Action:      ActionNewsletterDeactivated,
		EntityType:  ptr("newsletter"),
		EntityID:    &id,
	})

	w.WriteHeader(http.StatusNoContent)
}

// handleExportNewsletter is a pure read: families with an 'arrived'
// appointment in the window, one row per family (its lowest-id guardian
// represents it, matching legacy's single-guardian-per-row mail-merge
// shape), optionally excluding families already marked as having
// received the given newsletter. No email column -- this is a physical
// mail-merge export, not an electronic newsletter.
func (s *Server) handleExportNewsletter(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	start, end, ok := queryDateRange(w, r)
	if !ok {
		return
	}
	newsletterID, ok := queryInt64Ptr(w, r, "newsletter_id")
	if !ok {
		return
	}

	families, err := s.queries.ListEligibleFamiliesForNewsletter(r.Context(), db.ListEligibleFamiliesForNewsletterParams{
		LabID: labID, StartDate: start, EndDate: end, NewsletterID: newsletterID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	// Buffered rather than streamed directly to w: csv.Writer wraps a
	// bufio.Writer, so nothing would actually reach the client until the
	// buffer fills or Flush runs anyway -- buffering fully first means an
	// encode failure can still become a real error response instead of a
	// silently truncated 200 (headers are only written once we know the
	// whole body encoded cleanly).
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	for _, f := range families {
		if err := writer.Write([]string{f.FirstName, f.LastName, f.Address, f.City, f.State, f.Zip}); err != nil {
			s.logger.Error("encode newsletter export row", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		s.logger.Error("flush newsletter export csv", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=newsletters.csv")
	w.Write(buf.Bytes())
}

// handleMarkNewsletterSent re-runs the same eligible-family query the
// export above uses (with this newsletter as the exclusion filter) and
// records each guardian as sent.  This is a separate explicit action rather
// than a side effect on the read-only export, unlike legacy's
// export-form checkbox.
func (s *Server) handleMarkNewsletterSent(w http.ResponseWriter, r *http.Request) {
	newsletterID, ok := idParam(w, r, "newsletterID")
	if !ok {
		return
	}
	newsletter, err := s.queries.GetNewsletterByID(r.Context(), newsletterID)
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	start, end, ok := queryDateRange(w, r)
	if !ok {
		return
	}

	families, err := s.queries.ListEligibleFamiliesForNewsletter(r.Context(), db.ListEligibleFamiliesForNewsletterParams{
		LabID: newsletter.LabID, StartDate: start, EndDate: end, NewsletterID: &newsletterID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	for _, f := range families {
		if err := s.queries.MarkNewsletterSent(r.Context(), db.MarkNewsletterSentParams{
			NewsletterID: newsletterID, GuardianID: f.GuardianID,
		}); err != nil {
			s.writeDBError(w, err)
			return
		}
	}

	s.recordAuditEvent(r, audit.Event{
		ActorUserID: currentUserID(r.Context()),
		LabID:       &newsletter.LabID,
		Action:      ActionNewsletterMarkedSent,
		EntityType:  ptr("newsletter"),
		EntityID:    &newsletterID,
		Metadata:    map[string]int{"family_count": len(families)},
	})

	writeJSON(w, http.StatusOK, map[string]int{"marked_sent": len(families)})
}
