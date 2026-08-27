package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/db/dbfake"
)

func TestHandleAddExperimentCondition_Success(t *testing.T) {
	var captured db.AddExperimentConditionParams
	var capturedAudit db.CreateAuditEventParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		AddExperimentConditionFunc: func(ctx context.Context, arg db.AddExperimentConditionParams) error {
			captured = arg
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			capturedAudit = arg
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/conditions/", cookie, addConditionRequest{ConditionID: 5})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.ExperimentID != 3 || captured.ConditionID != 5 {
		t.Errorf("AddExperimentCondition params = %+v", captured)
	}
	if capturedAudit.Action != ActionExperimentConditionAdded {
		t.Errorf("audit action = %q, want %q", capturedAudit.Action, ActionExperimentConditionAdded)
	}
}

func TestHandleAddExperimentCondition_RequiresAuth(t *testing.T) {
	s := newTestServer(&dbfake.Querier{})

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/conditions/", nil, addConditionRequest{ConditionID: 5})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleAddExperimentCondition_InvalidExperimentID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/not-a-number/conditions/", cookie, addConditionRequest{ConditionID: 5})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleAddExperimentCondition_UnknownCondition(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		AddExperimentConditionFunc: func(ctx context.Context, arg db.AddExperimentConditionParams) error {
			return &pgconn.PgError{Code: pgForeignKeyViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/conditions/", cookie, addConditionRequest{ConditionID: 404})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleAddExperimentCondition_AlreadyAdded(t *testing.T) {
	// experiment_conditions has a composite primary key on
	// (experiment_id, condition_id); adding the same pair twice is a
	// unique violation, not a foreign key or check violation.
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		AddExperimentConditionFunc: func(ctx context.Context, arg db.AddExperimentConditionParams) error {
			return &pgconn.PgError{Code: pgUniqueViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/conditions/", cookie, addConditionRequest{ConditionID: 5})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandleAddExperimentCondition_UnexpectedDBError(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		AddExperimentConditionFunc: func(ctx context.Context, arg db.AddExperimentConditionParams) error {
			return assertErr("connection reset by peer")
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/conditions/", cookie, addConditionRequest{ConditionID: 5})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleRemoveExperimentCondition_InvalidExperimentID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodDelete, "/experiments/not-a-number/conditions/5", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRemoveExperimentCondition_InvalidConditionID(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/experiments/3/conditions/not-a-number", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleListExperimentConditions_InvalidExperimentID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/not-a-number/conditions/", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRemoveExperimentCondition_Success(t *testing.T) {
	var captured db.RemoveExperimentConditionParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		RemoveExperimentConditionFunc: func(ctx context.Context, arg db.RemoveExperimentConditionParams) error {
			captured = arg
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/experiments/3/conditions/5", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.ExperimentID != 3 || captured.ConditionID != 5 {
		t.Errorf("RemoveExperimentCondition params = %+v", captured)
	}
}

func TestHandleListExperimentConditions_Success(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		ListExperimentConditionsFunc: func(ctx context.Context, experimentID int64) ([]db.Condition, error) {
			return []db.Condition{{ID: 5, Name: "Stimulus Type"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/3/conditions/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]conditionResponse](t, rec)
	if len(got) != 1 || got[0].Name != "Stimulus Type" {
		t.Errorf("response = %+v", got)
	}
}

func TestHandleAddExperimentEquipment_Success(t *testing.T) {
	var captured db.AddExperimentEquipmentParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		AddExperimentEquipmentFunc: func(ctx context.Context, arg db.AddExperimentEquipmentParams) error {
			captured = arg
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/equipment/", cookie, addEquipmentRequest{EquipmentID: 8})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.ExperimentID != 3 || captured.EquipmentID != 8 {
		t.Errorf("AddExperimentEquipment params = %+v", captured)
	}
}

func TestHandleAddExperimentEquipment_AlreadyAdded(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		AddExperimentEquipmentFunc: func(ctx context.Context, arg db.AddExperimentEquipmentParams) error {
			return &pgconn.PgError{Code: pgUniqueViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/equipment/", cookie, addEquipmentRequest{EquipmentID: 8})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandleAddExperimentEquipment_InvalidExperimentID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/not-a-number/equipment/", cookie, addEquipmentRequest{EquipmentID: 8})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRemoveExperimentEquipment_InvalidEquipmentID(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/experiments/3/equipment/not-a-number", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRemoveExperimentEquipment_Success(t *testing.T) {
	var captured db.RemoveExperimentEquipmentParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		RemoveExperimentEquipmentFunc: func(ctx context.Context, arg db.RemoveExperimentEquipmentParams) error {
			captured = arg
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/experiments/3/equipment/8", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.ExperimentID != 3 || captured.EquipmentID != 8 {
		t.Errorf("RemoveExperimentEquipment params = %+v", captured)
	}
}

func TestHandleAddExperimentTrainingRequirement_Success(t *testing.T) {
	var captured db.AddExperimentTrainingRequirementParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		AddExperimentTrainingRequirementFunc: func(ctx context.Context, arg db.AddExperimentTrainingRequirementParams) error {
			captured = arg
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/training-requirements/", cookie, addTrainingRequirementRequest{ExperimentRoleID: 2})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.ExperimentID != 3 || captured.ExperimentRoleID != 2 {
		t.Errorf("AddExperimentTrainingRequirement params = %+v", captured)
	}
}

func TestHandleAddExperimentTrainingRequirement_AlreadyAdded(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		AddExperimentTrainingRequirementFunc: func(ctx context.Context, arg db.AddExperimentTrainingRequirementParams) error {
			return &pgconn.PgError{Code: pgUniqueViolation}
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/3/training-requirements/", cookie, addTrainingRequirementRequest{ExperimentRoleID: 2})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandleAddExperimentTrainingRequirement_InvalidExperimentID(t *testing.T) {
	s, cookie := newAuthenticatedTestServer(&dbfake.Querier{}, 7)

	rec := doRequest(t, s, http.MethodPost, "/experiments/not-a-number/training-requirements/", cookie, addTrainingRequirementRequest{ExperimentRoleID: 2})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRemoveExperimentTrainingRequirement_InvalidRoleID(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/experiments/3/training-requirements/not-a-number", cookie, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleRemoveExperimentTrainingRequirement_Success(t *testing.T) {
	var captured db.RemoveExperimentTrainingRequirementParams
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		RemoveExperimentTrainingRequirementFunc: func(ctx context.Context, arg db.RemoveExperimentTrainingRequirementParams) error {
			captured = arg
			return nil
		},
		CreateAuditEventFunc: func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
			return db.AuditEvent{ID: 1}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodDelete, "/experiments/3/training-requirements/2", cookie, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusNoContent, rec.Body)
	}
	if captured.ExperimentID != 3 || captured.ExperimentRoleID != 2 {
		t.Errorf("RemoveExperimentTrainingRequirement params = %+v", captured)
	}
}

func TestHandleListExperimentTrainingRequirements_Success(t *testing.T) {
	q := &dbfake.Querier{
		GetExperimentByIDFunc: func(ctx context.Context, id int64) (db.Experiment, error) {
			return db.Experiment{ID: id, LabID: 1}, nil
		},
		ListExperimentTrainingRequirementsFunc: func(ctx context.Context, experimentID int64) ([]db.ExperimentRole, error) {
			return []db.ExperimentRole{{ID: 2, Name: "Experimenter"}}, nil
		},
	}
	s, cookie := newAuthenticatedTestServer(q, 7)

	rec := doRequest(t, s, http.MethodGet, "/experiments/3/training-requirements/", cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body)
	}
	got := decodeBody[[]experimentRoleResponse](t, rec)
	if len(got) != 1 || got[0].Name != "Experimenter" {
		t.Errorf("response = %+v", got)
	}
}
