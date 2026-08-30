package httpapi

import (
	"net/http"

	"github.com/ggem/coglab-manager-go/internal/db"
)

// requireLabMember verifies the current user belongs to labID, writing a
// 404 (not 403 -- matching writeDBError's existing "don't reveal what
// exists" convention used everywhere else in this package) and returning
// ok=false if not. This checks membership only: role-based permissions
// within a lab (who can do what once they're a member) are a separate,
// not-yet-built concern.
func (s *Server) requireLabMember(w http.ResponseWriter, r *http.Request, labID int64) bool {
	userID, ok := s.requireCurrentUserID(w, r)
	if !ok {
		return false
	}
	if _, err := s.queries.GetLabMembership(r.Context(), db.GetLabMembershipParams{UserID: userID, LabID: labID}); err != nil {
		s.writeDBError(w, err)
		return false
	}
	return true
}

// requireLabMemberFromURL is for routes nested under /labs/{labID}/... --
// it checks the lab named directly in the URL.
func (s *Server) requireLabMemberFromURL(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		labID, ok := idParam(w, r, "labID")
		if !ok {
			return
		}
		if !s.requireLabMember(w, r, labID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// The remaining middlewares are for the flat /{resource}/{id} routes: a
// resource's lab isn't in the URL there, so each one resolves it from the
// resource itself (reusing the existing GetByID queries, which already
// return lab_id) before checking membership. Applied once per route group
// via r.Use, they also cover any nested sub-routes under that resource
// (e.g. a condition's /values, or an experiment's join-management routes).

func (s *Server) requireLabMemberForCondition(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "conditionID")
		if !ok {
			return
		}
		condition, err := s.queries.GetConditionByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, condition.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLabMemberForConditionValue(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "valueID")
		if !ok {
			return
		}
		labID, err := s.queries.GetConditionValueLabID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, labID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLabMemberForEquipment(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "equipmentID")
		if !ok {
			return
		}
		equipment, err := s.queries.GetEquipmentByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, equipment.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLabMemberForExperimentRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "roleID")
		if !ok {
			return
		}
		role, err := s.queries.GetExperimentRoleByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, role.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLabMemberForLabAvailabilityGeneral(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "availabilityID")
		if !ok {
			return
		}
		row, err := s.queries.GetLabAvailabilityGeneralByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, row.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLabMemberForLabAvailabilitySpecific(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "availabilityID")
		if !ok {
			return
		}
		row, err := s.queries.GetLabAvailabilitySpecificByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, row.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLabMemberForScheduleBlocking(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "blockingID")
		if !ok {
			return
		}
		blocking, err := s.queries.GetScheduleBlockingByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, blocking.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireLabMemberForAppointment gates /appointments/{appointmentID}/...:
// appointments has no lab_id of its own, so it's resolved via the
// experiment (GetAppointmentLabID).
func (s *Server) requireLabMemberForAppointment(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "appointmentID")
		if !ok {
			return
		}
		labID, err := s.queries.GetAppointmentLabID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, labID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireLabMemberForExperiment also gates the join-management routes
// nested under /experiments/{experimentID} (attaching conditions,
// equipment, and training roles): it checks the experiment's own lab, not
// whether the attached condition/equipment/role belongs to the same lab --
// cross-lab attachment is a data-integrity question distinct from request
// authorization, and isn't addressed here.
func (s *Server) requireLabMemberForExperiment(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "experimentID")
		if !ok {
			return
		}
		experiment, err := s.queries.GetExperimentByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, experiment.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLabMemberForProtocol(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "protocolID")
		if !ok {
			return
		}
		protocol, err := s.queries.GetProtocolByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, protocol.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLabMemberForGrant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "grantID")
		if !ok {
			return
		}
		grant, err := s.queries.GetGrantByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, grant.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLabMemberForZipCode(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "zipCodeID")
		if !ok {
			return
		}
		zipCode, err := s.queries.GetZipCodeByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, zipCode.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireLabMemberForNewsletter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := idParam(w, r, "newsletterID")
		if !ok {
			return
		}
		newsletter, err := s.queries.GetNewsletterByID(r.Context(), id)
		if err != nil {
			s.writeDBError(w, err)
			return
		}
		if !s.requireLabMember(w, r, newsletter.LabID) {
			return
		}
		next.ServeHTTP(w, r)
	})
}
