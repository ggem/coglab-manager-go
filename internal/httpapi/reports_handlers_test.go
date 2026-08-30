package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

// --- NIH ---

func TestHandleNIHReport_Success(t *testing.T) {
	var byCategoryArg db.NIHReportByCategoryParams
	var totalsArg db.NIHReportTotalsParams
	q := &dbfake.Querier{
		NIHReportByCategoryFunc: func(ctx context.Context, arg db.NIHReportByCategoryParams) ([]db.NIHReportByCategoryRow, error) {
			byCategoryArg = arg
			return []db.NIHReportByCategoryRow{
				{Category: "white", Male: 2, Female: 1, Unknown: 0},
				{Category: "asian", Male: 1, Female: 0, Unknown: 0},
			}, nil
		},
		NIHReportTotalsFunc: func(ctx context.Context, arg db.NIHReportTotalsParams) (db.NIHReportTotalsRow, error) {
			totalsArg = arg
			return db.NIHReportTotalsRow{Male: 3, Female: 1, Unknown: 0}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	grantID := int64(4)
	rec := doRequest(t, s, http.MethodGet, "/labs/9/reports/nih?start_date=2026-01-01&end_date=2026-02-01&grant_id=4", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	if byCategoryArg.LabID != 9 || byCategoryArg.GrantID == nil || *byCategoryArg.GrantID != grantID {
		t.Errorf("NIHReportByCategory params = %+v", byCategoryArg)
	}
	if totalsArg.LabID != 9 || totalsArg.GrantID == nil || *totalsArg.GrantID != grantID {
		t.Errorf("NIHReportTotals params = %+v", totalsArg)
	}
	got := decodeBody[nihReportResponse](t, rec)
	if len(got.Categories) != 2 || got.Categories[0].Category != "white" || got.Categories[0].Male != 2 {
		t.Errorf("Categories = %+v", got.Categories)
	}
	if got.Totals.Male != 3 || got.Totals.Female != 1 {
		t.Errorf("Totals = %+v", got.Totals)
	}
}

func TestHandleNIHReport_MissingDateRange(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/reports/nih?start_date=2026-01-01", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleNIHReport_ByCategoryUnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		NIHReportByCategoryFunc: func(ctx context.Context, arg db.NIHReportByCategoryParams) ([]db.NIHReportByCategoryRow, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/reports/nih?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleNIHReport_TotalsUnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		NIHReportByCategoryFunc: func(ctx context.Context, arg db.NIHReportByCategoryParams) ([]db.NIHReportByCategoryRow, error) {
			return nil, nil
		},
		NIHReportTotalsFunc: func(ctx context.Context, arg db.NIHReportTotalsParams) (db.NIHReportTotalsRow, error) {
			return db.NIHReportTotalsRow{}, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/reports/nih?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- HRC ---

func TestHandleHRCReport_Success(t *testing.T) {
	q := &dbfake.Querier{
		HRCReportByProtocolFunc: func(ctx context.Context, arg db.HRCReportByProtocolParams) ([]db.HRCReportByProtocolRow, error) {
			if arg.LabID != 9 {
				t.Errorf("HRCReportByProtocol LabID = %d, want 9", arg.LabID)
			}
			return []db.HRCReportByProtocolRow{
				{ProtocolID: 1, ProtocolName: "IRB-001", ChildCount: 3},
			}, nil
		},
		HRCReportTotalFunc: func(ctx context.Context, arg db.HRCReportTotalParams) (int64, error) {
			if arg.LabID != 9 {
				t.Errorf("HRCReportTotal LabID = %d, want 9", arg.LabID)
			}
			return 5, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/reports/hrc?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[hrcReportResponse](t, rec)
	if len(got.Protocols) != 1 || got.Protocols[0].ProtocolName != "IRB-001" || got.Protocols[0].ChildCount != 3 {
		t.Errorf("Protocols = %+v", got.Protocols)
	}
	if got.Total != 5 {
		t.Errorf("Total = %d, want 5", got.Total)
	}
}

func TestHandleHRCReport_MissingDateRange(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/reports/hrc", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleHRCReport_ByProtocolUnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		HRCReportByProtocolFunc: func(ctx context.Context, arg db.HRCReportByProtocolParams) ([]db.HRCReportByProtocolRow, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/reports/hrc?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleHRCReport_TotalUnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		HRCReportByProtocolFunc: func(ctx context.Context, arg db.HRCReportByProtocolParams) ([]db.HRCReportByProtocolRow, error) {
			return nil, nil
		},
		HRCReportTotalFunc: func(ctx context.Context, arg db.HRCReportTotalParams) (int64, error) {
			return 0, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/reports/hrc?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- Demographics ---

func TestHandleDemographicsReport_Success(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		DemographicsReportFunc: func(ctx context.Context, arg db.DemographicsReportParams) ([]db.DemographicsReportRow, error) {
			if arg.ExperimentID != 3 {
				t.Errorf("DemographicsReport ExperimentID = %d, want 3", arg.ExperimentID)
			}
			return []db.DemographicsReportRow{
				{ChildID: 1, FirstName: "A", Sex: "male", RaceEthnicity: []string{"white"}, ScheduleDate: birthDate(t, "2026-01-05"), AgeMonths: 12, GuardianEducation: "hs_grad_no_college"},
				{ChildID: 2, FirstName: "B", Sex: "female", RaceEthnicity: []string{"white", "asian"}, ScheduleDate: birthDate(t, "2026-01-10"), AgeMonths: 24, GuardianEducation: "hs_grad_no_college"},
				{ChildID: 3, FirstName: "C", Sex: "male", RaceEthnicity: []string{"asian"}, ScheduleDate: birthDate(t, "2026-01-15"), AgeMonths: 6, GuardianEducation: "degree_from_4yr_college_or_higher"},
			}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/3/reports/demographics?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[demographicsReportResponse](t, rec)
	if len(got.Children) != 3 {
		t.Fatalf("Children = %+v, want 3 rows", got.Children)
	}
	if got.Summary.Count != 3 {
		t.Errorf("Summary.Count = %d, want 3", got.Summary.Count)
	}
	if got.Summary.BySex["male"] != 2 || got.Summary.BySex["female"] != 1 {
		t.Errorf("Summary.BySex = %+v, want male=2 female=1", got.Summary.BySex)
	}
	if got.Summary.ByRaceEthnicity["white"] != 2 || got.Summary.ByRaceEthnicity["asian"] != 2 {
		t.Errorf("Summary.ByRaceEthnicity = %+v, want white=2 asian=2", got.Summary.ByRaceEthnicity)
	}
	if got.Summary.ByEducation["hs_grad_no_college"] != 2 || got.Summary.ByEducation["degree_from_4yr_college_or_higher"] != 1 {
		t.Errorf("Summary.ByEducation = %+v", got.Summary.ByEducation)
	}
	wantAvg := (12.0 + 24.0 + 6.0) / 3.0
	if got.Summary.AgeMonthsAvg != wantAvg {
		t.Errorf("Summary.AgeMonthsAvg = %v, want %v", got.Summary.AgeMonthsAvg, wantAvg)
	}
	if got.Summary.AgeMonthsMin != 6 {
		t.Errorf("Summary.AgeMonthsMin = %v, want 6", got.Summary.AgeMonthsMin)
	}
	if got.Summary.AgeMonthsMax != 24 {
		t.Errorf("Summary.AgeMonthsMax = %v, want 24", got.Summary.AgeMonthsMax)
	}
}

func TestHandleDemographicsReport_Empty(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		DemographicsReportFunc: func(ctx context.Context, arg db.DemographicsReportParams) ([]db.DemographicsReportRow, error) {
			return nil, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/3/reports/demographics?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[demographicsReportResponse](t, rec)
	if len(got.Children) != 0 {
		t.Errorf("Children = %+v, want none", got.Children)
	}
	if got.Summary.Count != 0 || got.Summary.AgeMonthsAvg != 0 || got.Summary.AgeMonthsMin != 0 || got.Summary.AgeMonthsMax != 0 {
		t.Errorf("Summary = %+v, want all zero", got.Summary)
	}
}

func TestHandleDemographicsReport_MissingDateRange(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/3/reports/demographics", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDemographicsReport_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		DemographicsReportFunc: func(ctx context.Context, arg db.DemographicsReportParams) ([]db.DemographicsReportRow, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/3/reports/demographics?start_date=2026-01-01&end_date=2026-02-01", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- Zip Codes ---

func TestHandleZipCodesReport_Success(t *testing.T) {
	q := &dbfake.Querier{
		ZipCodesReportFunc: func(ctx context.Context, arg db.ZipCodesReportParams) ([]db.ZipCodesReportRow, error) {
			if arg.LabID != 9 {
				t.Errorf("ZipCodesReport LabID = %d, want 9", arg.LabID)
			}
			priority := "high"
			return []db.ZipCodesReportRow{
				{Zip: "80301", Priority: &priority, ChildCount: 4},
				{Zip: "80302", Priority: nil, ChildCount: 1},
			}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/reports/zip-codes", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]zipCodesReportRow](t, rec)
	if len(got) != 2 {
		t.Fatalf("response = %+v, want 2 rows", got)
	}
	if got[0].Zip != "80301" || got[0].Priority == nil || *got[0].Priority != "high" || got[0].ChildCount != 4 {
		t.Errorf("row[0] = %+v", got[0])
	}
	if got[1].Zip != "80302" || got[1].Priority != nil || got[1].ChildCount != 1 {
		t.Errorf("row[1] = %+v, want nil Priority (unconfigured zip)", got[1])
	}
}

func TestHandleZipCodesReport_InvalidLabID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/not-a-number/reports/zip-codes", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleZipCodesReport_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		ZipCodesReportFunc: func(ctx context.Context, arg db.ZipCodesReportParams) ([]db.ZipCodesReportRow, error) {
			return nil, assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/labs/9/reports/zip-codes", cookie, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
