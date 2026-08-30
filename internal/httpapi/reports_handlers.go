package httpapi

import (
	"math"
	"net/http"

	"github.com/ggem/coglab-manager-go/internal/db"
)

type nihReportCategoryRow struct {
	Category string `json:"category"`
	Male     int64  `json:"male"`
	Female   int64  `json:"female"`
	Unknown  int64  `json:"unknown"`
}

type nihReportResponse struct {
	Categories []nihReportCategoryRow `json:"categories"`
	// Totals is distinct-child counts by sex -- not a sum of Categories,
	// since a child selecting more than one race_ethnicity category is
	// counted in each of that category's rows.
	Totals nihReportCategoryRow `json:"totals"`
}

func (s *Server) handleNIHReport(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	start, end, ok := queryDateRange(w, r)
	if !ok {
		return
	}
	grantID, ok := queryInt64Ptr(w, r, "grant_id")
	if !ok {
		return
	}

	byCategory, err := s.queries.NIHReportByCategory(r.Context(), db.NIHReportByCategoryParams{
		LabID: labID, StartDate: start, EndDate: end, GrantID: grantID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	totals, err := s.queries.NIHReportTotals(r.Context(), db.NIHReportTotalsParams{
		LabID: labID, StartDate: start, EndDate: end, GrantID: grantID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := nihReportResponse{
		Categories: make([]nihReportCategoryRow, len(byCategory)),
		Totals:     nihReportCategoryRow{Male: totals.Male, Female: totals.Female, Unknown: totals.Unknown},
	}
	for i, c := range byCategory {
		resp.Categories[i] = nihReportCategoryRow{Category: c.Category, Male: c.Male, Female: c.Female, Unknown: c.Unknown}
	}
	writeJSON(w, http.StatusOK, resp)
}

type hrcReportProtocolRow struct {
	ProtocolID   *int64 `json:"protocol_id"`
	ProtocolName string `json:"protocol_name"`
	ChildCount   int64  `json:"child_count"`
}

type hrcReportResponse struct {
	Protocols []hrcReportProtocolRow `json:"protocols"`
	Total     int64                  `json:"total"`
}

func (s *Server) handleHRCReport(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	start, end, ok := queryDateRange(w, r)
	if !ok {
		return
	}

	byProtocol, err := s.queries.HRCReportByProtocol(r.Context(), db.HRCReportByProtocolParams{
		LabID: labID, StartDate: start, EndDate: end,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}
	total, err := s.queries.HRCReportTotal(r.Context(), db.HRCReportTotalParams{
		LabID: labID, StartDate: start, EndDate: end,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := hrcReportResponse{
		Protocols: make([]hrcReportProtocolRow, len(byProtocol)),
		Total:     total,
	}
	for i, p := range byProtocol {
		resp.Protocols[i] = hrcReportProtocolRow{ProtocolID: &p.ProtocolID, ProtocolName: p.ProtocolName, ChildCount: p.ChildCount}
	}
	writeJSON(w, http.StatusOK, resp)
}

type demographicsReportChildRow struct {
	ChildID           int64    `json:"child_id"`
	FirstName         string   `json:"first_name"`
	LastName          string   `json:"last_name"`
	Sex               string   `json:"sex"`
	RaceEthnicity     []string `json:"race_ethnicity"`
	ScheduleDate      string   `json:"schedule_date"`
	AgeMonths         float64  `json:"age_months"`
	GuardianEducation string   `json:"guardian_education"`
}

type demographicsReportSummary struct {
	Count           int            `json:"count"`
	BySex           map[string]int `json:"by_sex"`
	ByRaceEthnicity map[string]int `json:"by_race_ethnicity"`
	ByEducation     map[string]int `json:"by_guardian_education"`
	AgeMonthsAvg    float64        `json:"age_months_avg"`
	AgeMonthsMin    float64        `json:"age_months_min"`
	AgeMonthsMax    float64        `json:"age_months_max"`
}

type demographicsReportResponse struct {
	Children []demographicsReportChildRow `json:"children"`
	Summary  demographicsReportSummary    `json:"summary"`
}

func (s *Server) handleDemographicsReport(w http.ResponseWriter, r *http.Request) {
	experimentID, ok := idParam(w, r, "experimentID")
	if !ok {
		return
	}
	start, end, ok := queryDateRange(w, r)
	if !ok {
		return
	}

	rows, err := s.queries.DemographicsReport(r.Context(), db.DemographicsReportParams{
		ExperimentID: experimentID, StartDate: start, EndDate: end,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := demographicsReportResponse{
		Children: make([]demographicsReportChildRow, len(rows)),
		Summary: demographicsReportSummary{
			BySex:           map[string]int{},
			ByRaceEthnicity: map[string]int{},
			ByEducation:     map[string]int{},
		},
	}
	// Seeded at the identity values for min/max (anything real is less
	// than +Inf, greater than -Inf) so the loop doesn't need an i == 0
	// check -- reset back to 0 below if there turn out to be no rows at
	// all, since encoding/json can't marshal Inf and writeJSON ignores
	// encode errors, so a genuinely empty report would otherwise come
	// back as a silently truncated body.
	resp.Summary.AgeMonthsMin = math.Inf(1)
	resp.Summary.AgeMonthsMax = math.Inf(-1)
	var ageSum float64
	for i, row := range rows {
		resp.Children[i] = demographicsReportChildRow{
			ChildID: row.ChildID, FirstName: row.FirstName, LastName: row.LastName,
			Sex: row.Sex, RaceEthnicity: row.RaceEthnicity,
			ScheduleDate: row.ScheduleDate.Time.Format(dateLayout), AgeMonths: row.AgeMonths,
			GuardianEducation: row.GuardianEducation,
		}
		resp.Summary.BySex[row.Sex]++
		for _, category := range row.RaceEthnicity {
			resp.Summary.ByRaceEthnicity[category]++
		}
		resp.Summary.ByEducation[row.GuardianEducation]++
		ageSum += row.AgeMonths
		resp.Summary.AgeMonthsMin = min(resp.Summary.AgeMonthsMin, row.AgeMonths)
		resp.Summary.AgeMonthsMax = max(resp.Summary.AgeMonthsMax, row.AgeMonths)
	}
	resp.Summary.Count = len(rows)
	if len(rows) > 0 {
		resp.Summary.AgeMonthsAvg = ageSum / float64(len(rows))
	} else {
		resp.Summary.AgeMonthsMin = 0
		resp.Summary.AgeMonthsMax = 0
	}

	writeJSON(w, http.StatusOK, resp)
}

type zipCodesReportRow struct {
	Zip        string  `json:"zip"`
	Priority   *string `json:"priority"`
	ChildCount int64   `json:"child_count"`
}

func (s *Server) handleZipCodesReport(w http.ResponseWriter, r *http.Request) {
	labID, ok := idParam(w, r, "labID")
	if !ok {
		return
	}
	recruitmentSourceID, ok := queryInt64Ptr(w, r, "recruitment_source_id")
	if !ok {
		return
	}

	rows, err := s.queries.ZipCodesReport(r.Context(), db.ZipCodesReportParams{
		LabID: labID, RecruitmentSourceID: recruitmentSourceID,
	})
	if err != nil {
		s.writeDBError(w, err)
		return
	}

	resp := make([]zipCodesReportRow, len(rows))
	for i, row := range rows {
		resp[i] = zipCodesReportRow{Zip: row.Zip, Priority: row.Priority, ChildCount: row.ChildCount}
	}
	writeJSON(w, http.StatusOK, resp)
}
