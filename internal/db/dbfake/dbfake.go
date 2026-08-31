// Package dbfake provides a fake implementation of db.Querier for tests,
// so packages that depend on db.Querier can be tested without a real
// database connection.
package dbfake

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
)

// Querier implements db.Querier with each method backed by an optional,
// exported function field. Calling a method whose field is unset panics
// with a clear message, so a test exercising a code path it didn't expect
// fails loudly instead of returning a silent zero value.
type Querier struct {
	CreateUserFunc            func(ctx context.Context, arg db.CreateUserParams) (db.User, error)
	GetUserByEmailFunc        func(ctx context.Context, email string) (db.User, error)
	GetUserByIDFunc           func(ctx context.Context, id int64) (db.User, error)
	CreateSessionFunc         func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error)
	GetSessionByTokenHashFunc func(ctx context.Context, tokenHash []byte) (db.Session, error)
	RevokeSessionFunc         func(ctx context.Context, tokenHash []byte) error
	TouchSessionLastSeenFunc  func(ctx context.Context, id int64) error
	CreateAuditEventFunc      func(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error)

	CreateFamilyFunc   func(ctx context.Context, arg db.CreateFamilyParams) (db.Family, error)
	GetFamilyByIDFunc  func(ctx context.Context, id int64) (db.Family, error)
	UpdateFamilyFunc   func(ctx context.Context, arg db.UpdateFamilyParams) (db.Family, error)
	SearchFamiliesFunc func(ctx context.Context, arg db.SearchFamiliesParams) ([]db.Family, error)

	CreateGuardianFunc        func(ctx context.Context, arg db.CreateGuardianParams) (db.Guardian, error)
	GetGuardianByIDFunc       func(ctx context.Context, id int64) (db.Guardian, error)
	ListGuardiansByFamilyFunc func(ctx context.Context, familyID int64) ([]db.Guardian, error)
	UpdateGuardianFunc        func(ctx context.Context, arg db.UpdateGuardianParams) (db.Guardian, error)
	DeleteGuardianFunc        func(ctx context.Context, id int64) error

	CreateChildFunc          func(ctx context.Context, arg db.CreateChildParams) (db.Child, error)
	GetChildByIDFunc         func(ctx context.Context, id int64) (db.Child, error)
	ListChildrenByFamilyFunc func(ctx context.Context, familyID int64) ([]db.Child, error)
	UpdateChildFunc          func(ctx context.Context, arg db.UpdateChildParams) (db.Child, error)
	DeactivateChildFunc      func(ctx context.Context, arg db.DeactivateChildParams) error
	SearchChildrenFunc       func(ctx context.Context, arg db.SearchChildrenParams) ([]db.Child, error)

	CreateNoteFunc        func(ctx context.Context, arg db.CreateNoteParams) (db.Note, error)
	ListNotesByEntityFunc func(ctx context.Context, arg db.ListNotesByEntityParams) ([]db.Note, error)

	ListActiveRecruitmentSourcesFunc func(ctx context.Context) ([]db.RecruitmentSource, error)
	CreateRecruitmentSourceFunc      func(ctx context.Context, name string) (db.RecruitmentSource, error)

	CreateExperimentFunc     func(ctx context.Context, arg db.CreateExperimentParams) (db.Experiment, error)
	GetExperimentByIDFunc    func(ctx context.Context, id int64) (db.Experiment, error)
	ListExperimentsByLabFunc func(ctx context.Context, labID int64) ([]db.Experiment, error)
	UpdateExperimentFunc     func(ctx context.Context, arg db.UpdateExperimentParams) (db.Experiment, error)
	DeactivateExperimentFunc func(ctx context.Context, id int64) error

	AddExperimentConditionFunc              func(ctx context.Context, arg db.AddExperimentConditionParams) error
	RemoveExperimentConditionFunc           func(ctx context.Context, arg db.RemoveExperimentConditionParams) error
	ListExperimentConditionsFunc            func(ctx context.Context, experimentID int64) ([]db.Condition, error)
	AddExperimentEquipmentFunc              func(ctx context.Context, arg db.AddExperimentEquipmentParams) error
	RemoveExperimentEquipmentFunc           func(ctx context.Context, arg db.RemoveExperimentEquipmentParams) error
	ListExperimentEquipmentFunc             func(ctx context.Context, experimentID int64) ([]db.Equipment, error)
	AddExperimentTrainingRequirementFunc    func(ctx context.Context, arg db.AddExperimentTrainingRequirementParams) error
	RemoveExperimentTrainingRequirementFunc func(ctx context.Context, arg db.RemoveExperimentTrainingRequirementParams) error
	ListExperimentTrainingRequirementsFunc  func(ctx context.Context, experimentID int64) ([]db.ExperimentRole, error)
	AddExperimentGrantFunc                  func(ctx context.Context, arg db.AddExperimentGrantParams) error
	RemoveExperimentGrantFunc               func(ctx context.Context, arg db.RemoveExperimentGrantParams) error
	ListExperimentGrantsFunc                func(ctx context.Context, experimentID int64) ([]db.Grant, error)

	CreateProtocolFunc     func(ctx context.Context, arg db.CreateProtocolParams) (db.Protocol, error)
	GetProtocolByIDFunc    func(ctx context.Context, id int64) (db.Protocol, error)
	ListProtocolsByLabFunc func(ctx context.Context, labID int64) ([]db.Protocol, error)
	UpdateProtocolFunc     func(ctx context.Context, arg db.UpdateProtocolParams) (db.Protocol, error)
	DeactivateProtocolFunc func(ctx context.Context, id int64) error

	CreateGrantFunc     func(ctx context.Context, arg db.CreateGrantParams) (db.Grant, error)
	GetGrantByIDFunc    func(ctx context.Context, id int64) (db.Grant, error)
	ListGrantsByLabFunc func(ctx context.Context, labID int64) ([]db.Grant, error)
	UpdateGrantFunc     func(ctx context.Context, arg db.UpdateGrantParams) (db.Grant, error)
	DeactivateGrantFunc func(ctx context.Context, id int64) error

	CreateZipCodeFunc     func(ctx context.Context, arg db.CreateZipCodeParams) (db.Zipcode, error)
	GetZipCodeByIDFunc    func(ctx context.Context, id int64) (db.Zipcode, error)
	ListZipCodesByLabFunc func(ctx context.Context, labID int64) ([]db.Zipcode, error)
	UpdateZipCodeFunc     func(ctx context.Context, arg db.UpdateZipCodeParams) (db.Zipcode, error)
	DeactivateZipCodeFunc func(ctx context.Context, id int64) error

	CreateNewsletterFunc                  func(ctx context.Context, arg db.CreateNewsletterParams) (db.Newsletter, error)
	GetNewsletterByIDFunc                 func(ctx context.Context, id int64) (db.Newsletter, error)
	ListNewslettersByLabFunc              func(ctx context.Context, labID int64) ([]db.Newsletter, error)
	DeactivateNewsletterFunc              func(ctx context.Context, id int64) error
	ListEligibleFamiliesForNewsletterFunc func(ctx context.Context, arg db.ListEligibleFamiliesForNewsletterParams) ([]db.ListEligibleFamiliesForNewsletterRow, error)
	MarkNewsletterSentFunc                func(ctx context.Context, arg db.MarkNewsletterSentParams) error

	ArriveAppointmentFunc func(ctx context.Context, id int64) (db.Appointment, error)

	NIHReportByCategoryFunc func(ctx context.Context, arg db.NIHReportByCategoryParams) ([]db.NIHReportByCategoryRow, error)
	NIHReportTotalsFunc     func(ctx context.Context, arg db.NIHReportTotalsParams) (db.NIHReportTotalsRow, error)
	HRCReportByProtocolFunc func(ctx context.Context, arg db.HRCReportByProtocolParams) ([]db.HRCReportByProtocolRow, error)
	HRCReportTotalFunc      func(ctx context.Context, arg db.HRCReportTotalParams) (int64, error)
	DemographicsReportFunc  func(ctx context.Context, arg db.DemographicsReportParams) ([]db.DemographicsReportRow, error)
	ZipCodesReportFunc      func(ctx context.Context, arg db.ZipCodesReportParams) ([]db.ZipCodesReportRow, error)

	GetJobLastRunFunc                       func(ctx context.Context, jobName string) (pgtype.Timestamptz, error)
	UpsertJobLastRunFunc                    func(ctx context.Context, jobName string) error
	ListChangedAppointmentIDsSinceFunc      func(ctx context.Context, arg db.ListChangedAppointmentIDsSinceParams) ([]int64, error)
	ListRecipientsForAppointmentsFunc       func(ctx context.Context, appointmentIDs []int64) ([]db.ListRecipientsForAppointmentsRow, error)
	ListPendingAppointmentsForUserInLabFunc func(ctx context.Context, arg db.ListPendingAppointmentsForUserInLabParams) ([]db.ListPendingAppointmentsForUserInLabRow, error)
	ListAppointmentsDueForReminderFunc      func(ctx context.Context, dueBefore pgtype.Timestamp) ([]db.ListAppointmentsDueForReminderRow, error)
	MarkAppointmentReminderSentFunc         func(ctx context.Context, id int64) error

	CreateConditionFunc                func(ctx context.Context, arg db.CreateConditionParams) (db.Condition, error)
	GetConditionByIDFunc               func(ctx context.Context, id int64) (db.Condition, error)
	ListConditionsByLabFunc            func(ctx context.Context, labID int64) ([]db.Condition, error)
	UpdateConditionFunc                func(ctx context.Context, arg db.UpdateConditionParams) (db.Condition, error)
	DeactivateConditionFunc            func(ctx context.Context, id int64) error
	CreateConditionValueFunc           func(ctx context.Context, arg db.CreateConditionValueParams) (db.ConditionValue, error)
	GetConditionValueLabIDFunc         func(ctx context.Context, id int64) (int64, error)
	ListConditionValuesByConditionFunc func(ctx context.Context, conditionID int64) ([]db.ConditionValue, error)
	UpdateConditionValueFunc           func(ctx context.Context, arg db.UpdateConditionValueParams) (db.ConditionValue, error)
	DeactivateConditionValueFunc       func(ctx context.Context, id int64) error

	GetLabMembershipFunc func(ctx context.Context, arg db.GetLabMembershipParams) (db.LabMembership, error)
	ListLabsForUserFunc  func(ctx context.Context, userID int64) ([]db.Lab, error)

	CreateEquipmentFunc     func(ctx context.Context, arg db.CreateEquipmentParams) (db.Equipment, error)
	GetEquipmentByIDFunc    func(ctx context.Context, id int64) (db.Equipment, error)
	ListEquipmentByLabFunc  func(ctx context.Context, labID int64) ([]db.Equipment, error)
	UpdateEquipmentFunc     func(ctx context.Context, arg db.UpdateEquipmentParams) (db.Equipment, error)
	DeactivateEquipmentFunc func(ctx context.Context, id int64) error

	CreateExperimentRoleFunc     func(ctx context.Context, arg db.CreateExperimentRoleParams) (db.ExperimentRole, error)
	GetExperimentRoleByIDFunc    func(ctx context.Context, id int64) (db.ExperimentRole, error)
	ListExperimentRolesByLabFunc func(ctx context.Context, labID int64) ([]db.ExperimentRole, error)
	UpdateExperimentRoleFunc     func(ctx context.Context, arg db.UpdateExperimentRoleParams) (db.ExperimentRole, error)
	DeactivateExperimentRoleFunc func(ctx context.Context, id int64) error
	SetExperimentRoleSitterFunc  func(ctx context.Context, arg db.SetExperimentRoleSitterParams) (db.ExperimentRole, error)
	GetSitterRoleForLabFunc      func(ctx context.Context, labID int64) (db.ExperimentRole, error)

	AddLabMemberTrainingFunc          func(ctx context.Context, arg db.AddLabMemberTrainingParams) error
	RemoveLabMemberTrainingFunc       func(ctx context.Context, arg db.RemoveLabMemberTrainingParams) error
	ListLabMemberTrainingsForRoleFunc func(ctx context.Context, experimentRoleID int64) ([]db.User, error)
	ListLabMemberTrainingsForUserFunc func(ctx context.Context, userID int64) ([]db.ExperimentRole, error)

	CreateLabAvailabilityGeneralFunc     func(ctx context.Context, arg db.CreateLabAvailabilityGeneralParams) (db.LabAvailabilityGeneral, error)
	GetLabAvailabilityGeneralByIDFunc    func(ctx context.Context, id int64) (db.LabAvailabilityGeneral, error)
	ListLabAvailabilityGeneralByUserFunc func(ctx context.Context, arg db.ListLabAvailabilityGeneralByUserParams) ([]db.LabAvailabilityGeneral, error)
	DeactivateLabAvailabilityGeneralFunc func(ctx context.Context, id int64) error
	ListLabAvailabilityGeneralByLabFunc  func(ctx context.Context, labID int64) ([]db.LabAvailabilityGeneral, error)

	CreateLabAvailabilitySpecificFunc           func(ctx context.Context, arg db.CreateLabAvailabilitySpecificParams) (db.LabAvailabilitySpecific, error)
	GetLabAvailabilitySpecificByIDFunc          func(ctx context.Context, id int64) (db.LabAvailabilitySpecific, error)
	ListLabAvailabilitySpecificByUserFunc       func(ctx context.Context, arg db.ListLabAvailabilitySpecificByUserParams) ([]db.LabAvailabilitySpecific, error)
	DeactivateLabAvailabilitySpecificFunc       func(ctx context.Context, id int64) error
	ListLabAvailabilitySpecificForDateRangeFunc func(ctx context.Context, arg db.ListLabAvailabilitySpecificForDateRangeParams) ([]db.LabAvailabilitySpecific, error)

	CreateScheduleBlockingFunc            func(ctx context.Context, arg db.CreateScheduleBlockingParams) (db.ScheduleBlocking, error)
	GetScheduleBlockingByIDFunc           func(ctx context.Context, id int64) (db.ScheduleBlocking, error)
	ListScheduleBlockingsByLabFunc        func(ctx context.Context, labID int64) ([]db.ScheduleBlocking, error)
	ListScheduleBlockingsForDateRangeFunc func(ctx context.Context, arg db.ListScheduleBlockingsForDateRangeParams) ([]db.ScheduleBlocking, error)
	DeactivateScheduleBlockingFunc        func(ctx context.Context, id int64) error

	CreateAppointmentFunc            func(ctx context.Context, arg db.CreateAppointmentParams) (db.Appointment, error)
	GetAppointmentByIDFunc           func(ctx context.Context, id int64) (db.Appointment, error)
	GetAppointmentLabIDFunc          func(ctx context.Context, id int64) (int64, error)
	ListAppointmentsByExperimentFunc func(ctx context.Context, arg db.ListAppointmentsByExperimentParams) ([]db.Appointment, error)
	ScheduleAppointmentFunc          func(ctx context.Context, arg db.ScheduleAppointmentParams) (db.Appointment, error)
	ReleaseAppointmentFunc           func(ctx context.Context, id int64) (db.Appointment, error)

	CreateAppointmentExperimenterFunc                func(ctx context.Context, arg db.CreateAppointmentExperimenterParams) (db.AppointmentExperimenter, error)
	ListAppointmentExperimentersFunc                 func(ctx context.Context, appointmentID int64) ([]db.AppointmentExperimenter, error)
	ListBusyAppointmentExperimentersForDateRangeFunc func(ctx context.Context, arg db.ListBusyAppointmentExperimentersForDateRangeParams) ([]db.ListBusyAppointmentExperimentersForDateRangeRow, error)
	ListBusyEquipmentForDateRangeFunc                func(ctx context.Context, arg db.ListBusyEquipmentForDateRangeParams) ([]db.ListBusyEquipmentForDateRangeRow, error)

	ListEligibleChildrenForExperimentFunc func(ctx context.Context, arg db.ListEligibleChildrenForExperimentParams) ([]db.Child, error)
}

var _ db.Querier = (*Querier)(nil)

func (q *Querier) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	if q.CreateUserFunc == nil {
		panic("dbfake: CreateUser not implemented")
	}
	return q.CreateUserFunc(ctx, arg)
}

func (q *Querier) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	if q.GetUserByEmailFunc == nil {
		panic("dbfake: GetUserByEmail not implemented")
	}
	return q.GetUserByEmailFunc(ctx, email)
}

func (q *Querier) GetUserByID(ctx context.Context, id int64) (db.User, error) {
	if q.GetUserByIDFunc == nil {
		panic("dbfake: GetUserByID not implemented")
	}
	return q.GetUserByIDFunc(ctx, id)
}

func (q *Querier) CreateSession(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
	if q.CreateSessionFunc == nil {
		panic("dbfake: CreateSession not implemented")
	}
	return q.CreateSessionFunc(ctx, arg)
}

func (q *Querier) GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (db.Session, error) {
	if q.GetSessionByTokenHashFunc == nil {
		panic("dbfake: GetSessionByTokenHash not implemented")
	}
	return q.GetSessionByTokenHashFunc(ctx, tokenHash)
}

func (q *Querier) RevokeSession(ctx context.Context, tokenHash []byte) error {
	if q.RevokeSessionFunc == nil {
		panic("dbfake: RevokeSession not implemented")
	}
	return q.RevokeSessionFunc(ctx, tokenHash)
}

func (q *Querier) TouchSessionLastSeen(ctx context.Context, id int64) error {
	if q.TouchSessionLastSeenFunc == nil {
		panic("dbfake: TouchSessionLastSeen not implemented")
	}
	return q.TouchSessionLastSeenFunc(ctx, id)
}

func (q *Querier) CreateAuditEvent(ctx context.Context, arg db.CreateAuditEventParams) (db.AuditEvent, error) {
	if q.CreateAuditEventFunc == nil {
		panic("dbfake: CreateAuditEvent not implemented")
	}
	return q.CreateAuditEventFunc(ctx, arg)
}

func (q *Querier) CreateFamily(ctx context.Context, arg db.CreateFamilyParams) (db.Family, error) {
	if q.CreateFamilyFunc == nil {
		panic("dbfake: CreateFamily not implemented")
	}
	return q.CreateFamilyFunc(ctx, arg)
}

func (q *Querier) GetFamilyByID(ctx context.Context, id int64) (db.Family, error) {
	if q.GetFamilyByIDFunc == nil {
		panic("dbfake: GetFamilyByID not implemented")
	}
	return q.GetFamilyByIDFunc(ctx, id)
}

func (q *Querier) UpdateFamily(ctx context.Context, arg db.UpdateFamilyParams) (db.Family, error) {
	if q.UpdateFamilyFunc == nil {
		panic("dbfake: UpdateFamily not implemented")
	}
	return q.UpdateFamilyFunc(ctx, arg)
}

func (q *Querier) SearchFamilies(ctx context.Context, arg db.SearchFamiliesParams) ([]db.Family, error) {
	if q.SearchFamiliesFunc == nil {
		panic("dbfake: SearchFamilies not implemented")
	}
	return q.SearchFamiliesFunc(ctx, arg)
}

func (q *Querier) CreateGuardian(ctx context.Context, arg db.CreateGuardianParams) (db.Guardian, error) {
	if q.CreateGuardianFunc == nil {
		panic("dbfake: CreateGuardian not implemented")
	}
	return q.CreateGuardianFunc(ctx, arg)
}

func (q *Querier) GetGuardianByID(ctx context.Context, id int64) (db.Guardian, error) {
	if q.GetGuardianByIDFunc == nil {
		panic("dbfake: GetGuardianByID not implemented")
	}
	return q.GetGuardianByIDFunc(ctx, id)
}

func (q *Querier) ListGuardiansByFamily(ctx context.Context, familyID int64) ([]db.Guardian, error) {
	if q.ListGuardiansByFamilyFunc == nil {
		panic("dbfake: ListGuardiansByFamily not implemented")
	}
	return q.ListGuardiansByFamilyFunc(ctx, familyID)
}

func (q *Querier) UpdateGuardian(ctx context.Context, arg db.UpdateGuardianParams) (db.Guardian, error) {
	if q.UpdateGuardianFunc == nil {
		panic("dbfake: UpdateGuardian not implemented")
	}
	return q.UpdateGuardianFunc(ctx, arg)
}

func (q *Querier) DeleteGuardian(ctx context.Context, id int64) error {
	if q.DeleteGuardianFunc == nil {
		panic("dbfake: DeleteGuardian not implemented")
	}
	return q.DeleteGuardianFunc(ctx, id)
}

func (q *Querier) CreateChild(ctx context.Context, arg db.CreateChildParams) (db.Child, error) {
	if q.CreateChildFunc == nil {
		panic("dbfake: CreateChild not implemented")
	}
	return q.CreateChildFunc(ctx, arg)
}

func (q *Querier) GetChildByID(ctx context.Context, id int64) (db.Child, error) {
	if q.GetChildByIDFunc == nil {
		panic("dbfake: GetChildByID not implemented")
	}
	return q.GetChildByIDFunc(ctx, id)
}

func (q *Querier) ListChildrenByFamily(ctx context.Context, familyID int64) ([]db.Child, error) {
	if q.ListChildrenByFamilyFunc == nil {
		panic("dbfake: ListChildrenByFamily not implemented")
	}
	return q.ListChildrenByFamilyFunc(ctx, familyID)
}

func (q *Querier) UpdateChild(ctx context.Context, arg db.UpdateChildParams) (db.Child, error) {
	if q.UpdateChildFunc == nil {
		panic("dbfake: UpdateChild not implemented")
	}
	return q.UpdateChildFunc(ctx, arg)
}

func (q *Querier) DeactivateChild(ctx context.Context, arg db.DeactivateChildParams) error {
	if q.DeactivateChildFunc == nil {
		panic("dbfake: DeactivateChild not implemented")
	}
	return q.DeactivateChildFunc(ctx, arg)
}

func (q *Querier) SearchChildren(ctx context.Context, arg db.SearchChildrenParams) ([]db.Child, error) {
	if q.SearchChildrenFunc == nil {
		panic("dbfake: SearchChildren not implemented")
	}
	return q.SearchChildrenFunc(ctx, arg)
}

func (q *Querier) CreateNote(ctx context.Context, arg db.CreateNoteParams) (db.Note, error) {
	if q.CreateNoteFunc == nil {
		panic("dbfake: CreateNote not implemented")
	}
	return q.CreateNoteFunc(ctx, arg)
}

func (q *Querier) ListNotesByEntity(ctx context.Context, arg db.ListNotesByEntityParams) ([]db.Note, error) {
	if q.ListNotesByEntityFunc == nil {
		panic("dbfake: ListNotesByEntity not implemented")
	}
	return q.ListNotesByEntityFunc(ctx, arg)
}

func (q *Querier) ListActiveRecruitmentSources(ctx context.Context) ([]db.RecruitmentSource, error) {
	if q.ListActiveRecruitmentSourcesFunc == nil {
		panic("dbfake: ListActiveRecruitmentSources not implemented")
	}
	return q.ListActiveRecruitmentSourcesFunc(ctx)
}

func (q *Querier) CreateRecruitmentSource(ctx context.Context, name string) (db.RecruitmentSource, error) {
	if q.CreateRecruitmentSourceFunc == nil {
		panic("dbfake: CreateRecruitmentSource not implemented")
	}
	return q.CreateRecruitmentSourceFunc(ctx, name)
}

func (q *Querier) CreateExperiment(ctx context.Context, arg db.CreateExperimentParams) (db.Experiment, error) {
	if q.CreateExperimentFunc == nil {
		panic("dbfake: CreateExperiment not implemented")
	}
	return q.CreateExperimentFunc(ctx, arg)
}

func (q *Querier) GetExperimentByID(ctx context.Context, id int64) (db.Experiment, error) {
	if q.GetExperimentByIDFunc == nil {
		panic("dbfake: GetExperimentByID not implemented")
	}
	return q.GetExperimentByIDFunc(ctx, id)
}

func (q *Querier) ListExperimentsByLab(ctx context.Context, labID int64) ([]db.Experiment, error) {
	if q.ListExperimentsByLabFunc == nil {
		panic("dbfake: ListExperimentsByLab not implemented")
	}
	return q.ListExperimentsByLabFunc(ctx, labID)
}

func (q *Querier) UpdateExperiment(ctx context.Context, arg db.UpdateExperimentParams) (db.Experiment, error) {
	if q.UpdateExperimentFunc == nil {
		panic("dbfake: UpdateExperiment not implemented")
	}
	return q.UpdateExperimentFunc(ctx, arg)
}

func (q *Querier) DeactivateExperiment(ctx context.Context, id int64) error {
	if q.DeactivateExperimentFunc == nil {
		panic("dbfake: DeactivateExperiment not implemented")
	}
	return q.DeactivateExperimentFunc(ctx, id)
}

func (q *Querier) AddExperimentCondition(ctx context.Context, arg db.AddExperimentConditionParams) error {
	if q.AddExperimentConditionFunc == nil {
		panic("dbfake: AddExperimentCondition not implemented")
	}
	return q.AddExperimentConditionFunc(ctx, arg)
}

func (q *Querier) RemoveExperimentCondition(ctx context.Context, arg db.RemoveExperimentConditionParams) error {
	if q.RemoveExperimentConditionFunc == nil {
		panic("dbfake: RemoveExperimentCondition not implemented")
	}
	return q.RemoveExperimentConditionFunc(ctx, arg)
}

func (q *Querier) ListExperimentConditions(ctx context.Context, experimentID int64) ([]db.Condition, error) {
	if q.ListExperimentConditionsFunc == nil {
		panic("dbfake: ListExperimentConditions not implemented")
	}
	return q.ListExperimentConditionsFunc(ctx, experimentID)
}

func (q *Querier) AddExperimentEquipment(ctx context.Context, arg db.AddExperimentEquipmentParams) error {
	if q.AddExperimentEquipmentFunc == nil {
		panic("dbfake: AddExperimentEquipment not implemented")
	}
	return q.AddExperimentEquipmentFunc(ctx, arg)
}

func (q *Querier) RemoveExperimentEquipment(ctx context.Context, arg db.RemoveExperimentEquipmentParams) error {
	if q.RemoveExperimentEquipmentFunc == nil {
		panic("dbfake: RemoveExperimentEquipment not implemented")
	}
	return q.RemoveExperimentEquipmentFunc(ctx, arg)
}

func (q *Querier) ListExperimentEquipment(ctx context.Context, experimentID int64) ([]db.Equipment, error) {
	if q.ListExperimentEquipmentFunc == nil {
		panic("dbfake: ListExperimentEquipment not implemented")
	}
	return q.ListExperimentEquipmentFunc(ctx, experimentID)
}

func (q *Querier) AddExperimentTrainingRequirement(ctx context.Context, arg db.AddExperimentTrainingRequirementParams) error {
	if q.AddExperimentTrainingRequirementFunc == nil {
		panic("dbfake: AddExperimentTrainingRequirement not implemented")
	}
	return q.AddExperimentTrainingRequirementFunc(ctx, arg)
}

func (q *Querier) RemoveExperimentTrainingRequirement(ctx context.Context, arg db.RemoveExperimentTrainingRequirementParams) error {
	if q.RemoveExperimentTrainingRequirementFunc == nil {
		panic("dbfake: RemoveExperimentTrainingRequirement not implemented")
	}
	return q.RemoveExperimentTrainingRequirementFunc(ctx, arg)
}

func (q *Querier) ListExperimentTrainingRequirements(ctx context.Context, experimentID int64) ([]db.ExperimentRole, error) {
	if q.ListExperimentTrainingRequirementsFunc == nil {
		panic("dbfake: ListExperimentTrainingRequirements not implemented")
	}
	return q.ListExperimentTrainingRequirementsFunc(ctx, experimentID)
}

func (q *Querier) AddExperimentGrant(ctx context.Context, arg db.AddExperimentGrantParams) error {
	if q.AddExperimentGrantFunc == nil {
		panic("dbfake: AddExperimentGrant not implemented")
	}
	return q.AddExperimentGrantFunc(ctx, arg)
}

func (q *Querier) RemoveExperimentGrant(ctx context.Context, arg db.RemoveExperimentGrantParams) error {
	if q.RemoveExperimentGrantFunc == nil {
		panic("dbfake: RemoveExperimentGrant not implemented")
	}
	return q.RemoveExperimentGrantFunc(ctx, arg)
}

func (q *Querier) ListExperimentGrants(ctx context.Context, experimentID int64) ([]db.Grant, error) {
	if q.ListExperimentGrantsFunc == nil {
		panic("dbfake: ListExperimentGrants not implemented")
	}
	return q.ListExperimentGrantsFunc(ctx, experimentID)
}

func (q *Querier) CreateProtocol(ctx context.Context, arg db.CreateProtocolParams) (db.Protocol, error) {
	if q.CreateProtocolFunc == nil {
		panic("dbfake: CreateProtocol not implemented")
	}
	return q.CreateProtocolFunc(ctx, arg)
}

func (q *Querier) GetProtocolByID(ctx context.Context, id int64) (db.Protocol, error) {
	if q.GetProtocolByIDFunc == nil {
		panic("dbfake: GetProtocolByID not implemented")
	}
	return q.GetProtocolByIDFunc(ctx, id)
}

func (q *Querier) ListProtocolsByLab(ctx context.Context, labID int64) ([]db.Protocol, error) {
	if q.ListProtocolsByLabFunc == nil {
		panic("dbfake: ListProtocolsByLab not implemented")
	}
	return q.ListProtocolsByLabFunc(ctx, labID)
}

func (q *Querier) UpdateProtocol(ctx context.Context, arg db.UpdateProtocolParams) (db.Protocol, error) {
	if q.UpdateProtocolFunc == nil {
		panic("dbfake: UpdateProtocol not implemented")
	}
	return q.UpdateProtocolFunc(ctx, arg)
}

func (q *Querier) DeactivateProtocol(ctx context.Context, id int64) error {
	if q.DeactivateProtocolFunc == nil {
		panic("dbfake: DeactivateProtocol not implemented")
	}
	return q.DeactivateProtocolFunc(ctx, id)
}

func (q *Querier) CreateGrant(ctx context.Context, arg db.CreateGrantParams) (db.Grant, error) {
	if q.CreateGrantFunc == nil {
		panic("dbfake: CreateGrant not implemented")
	}
	return q.CreateGrantFunc(ctx, arg)
}

func (q *Querier) GetGrantByID(ctx context.Context, id int64) (db.Grant, error) {
	if q.GetGrantByIDFunc == nil {
		panic("dbfake: GetGrantByID not implemented")
	}
	return q.GetGrantByIDFunc(ctx, id)
}

func (q *Querier) ListGrantsByLab(ctx context.Context, labID int64) ([]db.Grant, error) {
	if q.ListGrantsByLabFunc == nil {
		panic("dbfake: ListGrantsByLab not implemented")
	}
	return q.ListGrantsByLabFunc(ctx, labID)
}

func (q *Querier) UpdateGrant(ctx context.Context, arg db.UpdateGrantParams) (db.Grant, error) {
	if q.UpdateGrantFunc == nil {
		panic("dbfake: UpdateGrant not implemented")
	}
	return q.UpdateGrantFunc(ctx, arg)
}

func (q *Querier) DeactivateGrant(ctx context.Context, id int64) error {
	if q.DeactivateGrantFunc == nil {
		panic("dbfake: DeactivateGrant not implemented")
	}
	return q.DeactivateGrantFunc(ctx, id)
}

func (q *Querier) CreateZipCode(ctx context.Context, arg db.CreateZipCodeParams) (db.Zipcode, error) {
	if q.CreateZipCodeFunc == nil {
		panic("dbfake: CreateZipCode not implemented")
	}
	return q.CreateZipCodeFunc(ctx, arg)
}

func (q *Querier) GetZipCodeByID(ctx context.Context, id int64) (db.Zipcode, error) {
	if q.GetZipCodeByIDFunc == nil {
		panic("dbfake: GetZipCodeByID not implemented")
	}
	return q.GetZipCodeByIDFunc(ctx, id)
}

func (q *Querier) ListZipCodesByLab(ctx context.Context, labID int64) ([]db.Zipcode, error) {
	if q.ListZipCodesByLabFunc == nil {
		panic("dbfake: ListZipCodesByLab not implemented")
	}
	return q.ListZipCodesByLabFunc(ctx, labID)
}

func (q *Querier) UpdateZipCode(ctx context.Context, arg db.UpdateZipCodeParams) (db.Zipcode, error) {
	if q.UpdateZipCodeFunc == nil {
		panic("dbfake: UpdateZipCode not implemented")
	}
	return q.UpdateZipCodeFunc(ctx, arg)
}

func (q *Querier) DeactivateZipCode(ctx context.Context, id int64) error {
	if q.DeactivateZipCodeFunc == nil {
		panic("dbfake: DeactivateZipCode not implemented")
	}
	return q.DeactivateZipCodeFunc(ctx, id)
}

func (q *Querier) CreateNewsletter(ctx context.Context, arg db.CreateNewsletterParams) (db.Newsletter, error) {
	if q.CreateNewsletterFunc == nil {
		panic("dbfake: CreateNewsletter not implemented")
	}
	return q.CreateNewsletterFunc(ctx, arg)
}

func (q *Querier) GetNewsletterByID(ctx context.Context, id int64) (db.Newsletter, error) {
	if q.GetNewsletterByIDFunc == nil {
		panic("dbfake: GetNewsletterByID not implemented")
	}
	return q.GetNewsletterByIDFunc(ctx, id)
}

func (q *Querier) ListNewslettersByLab(ctx context.Context, labID int64) ([]db.Newsletter, error) {
	if q.ListNewslettersByLabFunc == nil {
		panic("dbfake: ListNewslettersByLab not implemented")
	}
	return q.ListNewslettersByLabFunc(ctx, labID)
}

func (q *Querier) DeactivateNewsletter(ctx context.Context, id int64) error {
	if q.DeactivateNewsletterFunc == nil {
		panic("dbfake: DeactivateNewsletter not implemented")
	}
	return q.DeactivateNewsletterFunc(ctx, id)
}

func (q *Querier) ListEligibleFamiliesForNewsletter(ctx context.Context, arg db.ListEligibleFamiliesForNewsletterParams) ([]db.ListEligibleFamiliesForNewsletterRow, error) {
	if q.ListEligibleFamiliesForNewsletterFunc == nil {
		panic("dbfake: ListEligibleFamiliesForNewsletter not implemented")
	}
	return q.ListEligibleFamiliesForNewsletterFunc(ctx, arg)
}

func (q *Querier) MarkNewsletterSent(ctx context.Context, arg db.MarkNewsletterSentParams) error {
	if q.MarkNewsletterSentFunc == nil {
		panic("dbfake: MarkNewsletterSent not implemented")
	}
	return q.MarkNewsletterSentFunc(ctx, arg)
}

func (q *Querier) ArriveAppointment(ctx context.Context, id int64) (db.Appointment, error) {
	if q.ArriveAppointmentFunc == nil {
		panic("dbfake: ArriveAppointment not implemented")
	}
	return q.ArriveAppointmentFunc(ctx, id)
}

func (q *Querier) NIHReportByCategory(ctx context.Context, arg db.NIHReportByCategoryParams) ([]db.NIHReportByCategoryRow, error) {
	if q.NIHReportByCategoryFunc == nil {
		panic("dbfake: NIHReportByCategory not implemented")
	}
	return q.NIHReportByCategoryFunc(ctx, arg)
}

func (q *Querier) NIHReportTotals(ctx context.Context, arg db.NIHReportTotalsParams) (db.NIHReportTotalsRow, error) {
	if q.NIHReportTotalsFunc == nil {
		panic("dbfake: NIHReportTotals not implemented")
	}
	return q.NIHReportTotalsFunc(ctx, arg)
}

func (q *Querier) HRCReportByProtocol(ctx context.Context, arg db.HRCReportByProtocolParams) ([]db.HRCReportByProtocolRow, error) {
	if q.HRCReportByProtocolFunc == nil {
		panic("dbfake: HRCReportByProtocol not implemented")
	}
	return q.HRCReportByProtocolFunc(ctx, arg)
}

func (q *Querier) HRCReportTotal(ctx context.Context, arg db.HRCReportTotalParams) (int64, error) {
	if q.HRCReportTotalFunc == nil {
		panic("dbfake: HRCReportTotal not implemented")
	}
	return q.HRCReportTotalFunc(ctx, arg)
}

func (q *Querier) DemographicsReport(ctx context.Context, arg db.DemographicsReportParams) ([]db.DemographicsReportRow, error) {
	if q.DemographicsReportFunc == nil {
		panic("dbfake: DemographicsReport not implemented")
	}
	return q.DemographicsReportFunc(ctx, arg)
}

func (q *Querier) ZipCodesReport(ctx context.Context, arg db.ZipCodesReportParams) ([]db.ZipCodesReportRow, error) {
	if q.ZipCodesReportFunc == nil {
		panic("dbfake: ZipCodesReport not implemented")
	}
	return q.ZipCodesReportFunc(ctx, arg)
}

func (q *Querier) CreateCondition(ctx context.Context, arg db.CreateConditionParams) (db.Condition, error) {
	if q.CreateConditionFunc == nil {
		panic("dbfake: CreateCondition not implemented")
	}
	return q.CreateConditionFunc(ctx, arg)
}

func (q *Querier) GetConditionByID(ctx context.Context, id int64) (db.Condition, error) {
	if q.GetConditionByIDFunc == nil {
		panic("dbfake: GetConditionByID not implemented")
	}
	return q.GetConditionByIDFunc(ctx, id)
}

func (q *Querier) GetConditionValueLabID(ctx context.Context, id int64) (int64, error) {
	if q.GetConditionValueLabIDFunc == nil {
		panic("dbfake: GetConditionValueLabID not implemented")
	}
	return q.GetConditionValueLabIDFunc(ctx, id)
}

func (q *Querier) GetLabMembership(ctx context.Context, arg db.GetLabMembershipParams) (db.LabMembership, error) {
	if q.GetLabMembershipFunc == nil {
		panic("dbfake: GetLabMembership not implemented")
	}
	return q.GetLabMembershipFunc(ctx, arg)
}

func (q *Querier) ListLabsForUser(ctx context.Context, userID int64) ([]db.Lab, error) {
	if q.ListLabsForUserFunc == nil {
		panic("dbfake: ListLabsForUser not implemented")
	}
	return q.ListLabsForUserFunc(ctx, userID)
}

func (q *Querier) ListConditionsByLab(ctx context.Context, labID int64) ([]db.Condition, error) {
	if q.ListConditionsByLabFunc == nil {
		panic("dbfake: ListConditionsByLab not implemented")
	}
	return q.ListConditionsByLabFunc(ctx, labID)
}

func (q *Querier) UpdateCondition(ctx context.Context, arg db.UpdateConditionParams) (db.Condition, error) {
	if q.UpdateConditionFunc == nil {
		panic("dbfake: UpdateCondition not implemented")
	}
	return q.UpdateConditionFunc(ctx, arg)
}

func (q *Querier) DeactivateCondition(ctx context.Context, id int64) error {
	if q.DeactivateConditionFunc == nil {
		panic("dbfake: DeactivateCondition not implemented")
	}
	return q.DeactivateConditionFunc(ctx, id)
}

func (q *Querier) CreateConditionValue(ctx context.Context, arg db.CreateConditionValueParams) (db.ConditionValue, error) {
	if q.CreateConditionValueFunc == nil {
		panic("dbfake: CreateConditionValue not implemented")
	}
	return q.CreateConditionValueFunc(ctx, arg)
}

func (q *Querier) ListConditionValuesByCondition(ctx context.Context, conditionID int64) ([]db.ConditionValue, error) {
	if q.ListConditionValuesByConditionFunc == nil {
		panic("dbfake: ListConditionValuesByCondition not implemented")
	}
	return q.ListConditionValuesByConditionFunc(ctx, conditionID)
}

func (q *Querier) UpdateConditionValue(ctx context.Context, arg db.UpdateConditionValueParams) (db.ConditionValue, error) {
	if q.UpdateConditionValueFunc == nil {
		panic("dbfake: UpdateConditionValue not implemented")
	}
	return q.UpdateConditionValueFunc(ctx, arg)
}

func (q *Querier) DeactivateConditionValue(ctx context.Context, id int64) error {
	if q.DeactivateConditionValueFunc == nil {
		panic("dbfake: DeactivateConditionValue not implemented")
	}
	return q.DeactivateConditionValueFunc(ctx, id)
}

func (q *Querier) CreateEquipment(ctx context.Context, arg db.CreateEquipmentParams) (db.Equipment, error) {
	if q.CreateEquipmentFunc == nil {
		panic("dbfake: CreateEquipment not implemented")
	}
	return q.CreateEquipmentFunc(ctx, arg)
}

func (q *Querier) GetEquipmentByID(ctx context.Context, id int64) (db.Equipment, error) {
	if q.GetEquipmentByIDFunc == nil {
		panic("dbfake: GetEquipmentByID not implemented")
	}
	return q.GetEquipmentByIDFunc(ctx, id)
}

func (q *Querier) ListEquipmentByLab(ctx context.Context, labID int64) ([]db.Equipment, error) {
	if q.ListEquipmentByLabFunc == nil {
		panic("dbfake: ListEquipmentByLab not implemented")
	}
	return q.ListEquipmentByLabFunc(ctx, labID)
}

func (q *Querier) UpdateEquipment(ctx context.Context, arg db.UpdateEquipmentParams) (db.Equipment, error) {
	if q.UpdateEquipmentFunc == nil {
		panic("dbfake: UpdateEquipment not implemented")
	}
	return q.UpdateEquipmentFunc(ctx, arg)
}

func (q *Querier) DeactivateEquipment(ctx context.Context, id int64) error {
	if q.DeactivateEquipmentFunc == nil {
		panic("dbfake: DeactivateEquipment not implemented")
	}
	return q.DeactivateEquipmentFunc(ctx, id)
}

func (q *Querier) CreateExperimentRole(ctx context.Context, arg db.CreateExperimentRoleParams) (db.ExperimentRole, error) {
	if q.CreateExperimentRoleFunc == nil {
		panic("dbfake: CreateExperimentRole not implemented")
	}
	return q.CreateExperimentRoleFunc(ctx, arg)
}

func (q *Querier) GetExperimentRoleByID(ctx context.Context, id int64) (db.ExperimentRole, error) {
	if q.GetExperimentRoleByIDFunc == nil {
		panic("dbfake: GetExperimentRoleByID not implemented")
	}
	return q.GetExperimentRoleByIDFunc(ctx, id)
}

func (q *Querier) ListExperimentRolesByLab(ctx context.Context, labID int64) ([]db.ExperimentRole, error) {
	if q.ListExperimentRolesByLabFunc == nil {
		panic("dbfake: ListExperimentRolesByLab not implemented")
	}
	return q.ListExperimentRolesByLabFunc(ctx, labID)
}

func (q *Querier) UpdateExperimentRole(ctx context.Context, arg db.UpdateExperimentRoleParams) (db.ExperimentRole, error) {
	if q.UpdateExperimentRoleFunc == nil {
		panic("dbfake: UpdateExperimentRole not implemented")
	}
	return q.UpdateExperimentRoleFunc(ctx, arg)
}

func (q *Querier) DeactivateExperimentRole(ctx context.Context, id int64) error {
	if q.DeactivateExperimentRoleFunc == nil {
		panic("dbfake: DeactivateExperimentRole not implemented")
	}
	return q.DeactivateExperimentRoleFunc(ctx, id)
}

func (q *Querier) SetExperimentRoleSitter(ctx context.Context, arg db.SetExperimentRoleSitterParams) (db.ExperimentRole, error) {
	if q.SetExperimentRoleSitterFunc == nil {
		panic("dbfake: SetExperimentRoleSitter not implemented")
	}
	return q.SetExperimentRoleSitterFunc(ctx, arg)
}

func (q *Querier) GetSitterRoleForLab(ctx context.Context, labID int64) (db.ExperimentRole, error) {
	if q.GetSitterRoleForLabFunc == nil {
		panic("dbfake: GetSitterRoleForLab not implemented")
	}
	return q.GetSitterRoleForLabFunc(ctx, labID)
}

func (q *Querier) AddLabMemberTraining(ctx context.Context, arg db.AddLabMemberTrainingParams) error {
	if q.AddLabMemberTrainingFunc == nil {
		panic("dbfake: AddLabMemberTraining not implemented")
	}
	return q.AddLabMemberTrainingFunc(ctx, arg)
}

func (q *Querier) RemoveLabMemberTraining(ctx context.Context, arg db.RemoveLabMemberTrainingParams) error {
	if q.RemoveLabMemberTrainingFunc == nil {
		panic("dbfake: RemoveLabMemberTraining not implemented")
	}
	return q.RemoveLabMemberTrainingFunc(ctx, arg)
}

func (q *Querier) ListLabMemberTrainingsForRole(ctx context.Context, experimentRoleID int64) ([]db.User, error) {
	if q.ListLabMemberTrainingsForRoleFunc == nil {
		panic("dbfake: ListLabMemberTrainingsForRole not implemented")
	}
	return q.ListLabMemberTrainingsForRoleFunc(ctx, experimentRoleID)
}

func (q *Querier) ListLabMemberTrainingsForUser(ctx context.Context, userID int64) ([]db.ExperimentRole, error) {
	if q.ListLabMemberTrainingsForUserFunc == nil {
		panic("dbfake: ListLabMemberTrainingsForUser not implemented")
	}
	return q.ListLabMemberTrainingsForUserFunc(ctx, userID)
}

func (q *Querier) CreateLabAvailabilityGeneral(ctx context.Context, arg db.CreateLabAvailabilityGeneralParams) (db.LabAvailabilityGeneral, error) {
	if q.CreateLabAvailabilityGeneralFunc == nil {
		panic("dbfake: CreateLabAvailabilityGeneral not implemented")
	}
	return q.CreateLabAvailabilityGeneralFunc(ctx, arg)
}

func (q *Querier) GetLabAvailabilityGeneralByID(ctx context.Context, id int64) (db.LabAvailabilityGeneral, error) {
	if q.GetLabAvailabilityGeneralByIDFunc == nil {
		panic("dbfake: GetLabAvailabilityGeneralByID not implemented")
	}
	return q.GetLabAvailabilityGeneralByIDFunc(ctx, id)
}

func (q *Querier) ListLabAvailabilityGeneralByUser(ctx context.Context, arg db.ListLabAvailabilityGeneralByUserParams) ([]db.LabAvailabilityGeneral, error) {
	if q.ListLabAvailabilityGeneralByUserFunc == nil {
		panic("dbfake: ListLabAvailabilityGeneralByUser not implemented")
	}
	return q.ListLabAvailabilityGeneralByUserFunc(ctx, arg)
}

func (q *Querier) DeactivateLabAvailabilityGeneral(ctx context.Context, id int64) error {
	if q.DeactivateLabAvailabilityGeneralFunc == nil {
		panic("dbfake: DeactivateLabAvailabilityGeneral not implemented")
	}
	return q.DeactivateLabAvailabilityGeneralFunc(ctx, id)
}

func (q *Querier) ListLabAvailabilityGeneralByLab(ctx context.Context, labID int64) ([]db.LabAvailabilityGeneral, error) {
	if q.ListLabAvailabilityGeneralByLabFunc == nil {
		panic("dbfake: ListLabAvailabilityGeneralByLab not implemented")
	}
	return q.ListLabAvailabilityGeneralByLabFunc(ctx, labID)
}

func (q *Querier) CreateLabAvailabilitySpecific(ctx context.Context, arg db.CreateLabAvailabilitySpecificParams) (db.LabAvailabilitySpecific, error) {
	if q.CreateLabAvailabilitySpecificFunc == nil {
		panic("dbfake: CreateLabAvailabilitySpecific not implemented")
	}
	return q.CreateLabAvailabilitySpecificFunc(ctx, arg)
}

func (q *Querier) GetLabAvailabilitySpecificByID(ctx context.Context, id int64) (db.LabAvailabilitySpecific, error) {
	if q.GetLabAvailabilitySpecificByIDFunc == nil {
		panic("dbfake: GetLabAvailabilitySpecificByID not implemented")
	}
	return q.GetLabAvailabilitySpecificByIDFunc(ctx, id)
}

func (q *Querier) ListLabAvailabilitySpecificByUser(ctx context.Context, arg db.ListLabAvailabilitySpecificByUserParams) ([]db.LabAvailabilitySpecific, error) {
	if q.ListLabAvailabilitySpecificByUserFunc == nil {
		panic("dbfake: ListLabAvailabilitySpecificByUser not implemented")
	}
	return q.ListLabAvailabilitySpecificByUserFunc(ctx, arg)
}

func (q *Querier) DeactivateLabAvailabilitySpecific(ctx context.Context, id int64) error {
	if q.DeactivateLabAvailabilitySpecificFunc == nil {
		panic("dbfake: DeactivateLabAvailabilitySpecific not implemented")
	}
	return q.DeactivateLabAvailabilitySpecificFunc(ctx, id)
}

func (q *Querier) ListLabAvailabilitySpecificForDateRange(ctx context.Context, arg db.ListLabAvailabilitySpecificForDateRangeParams) ([]db.LabAvailabilitySpecific, error) {
	if q.ListLabAvailabilitySpecificForDateRangeFunc == nil {
		panic("dbfake: ListLabAvailabilitySpecificForDateRange not implemented")
	}
	return q.ListLabAvailabilitySpecificForDateRangeFunc(ctx, arg)
}

func (q *Querier) CreateScheduleBlocking(ctx context.Context, arg db.CreateScheduleBlockingParams) (db.ScheduleBlocking, error) {
	if q.CreateScheduleBlockingFunc == nil {
		panic("dbfake: CreateScheduleBlocking not implemented")
	}
	return q.CreateScheduleBlockingFunc(ctx, arg)
}

func (q *Querier) GetScheduleBlockingByID(ctx context.Context, id int64) (db.ScheduleBlocking, error) {
	if q.GetScheduleBlockingByIDFunc == nil {
		panic("dbfake: GetScheduleBlockingByID not implemented")
	}
	return q.GetScheduleBlockingByIDFunc(ctx, id)
}

func (q *Querier) ListScheduleBlockingsByLab(ctx context.Context, labID int64) ([]db.ScheduleBlocking, error) {
	if q.ListScheduleBlockingsByLabFunc == nil {
		panic("dbfake: ListScheduleBlockingsByLab not implemented")
	}
	return q.ListScheduleBlockingsByLabFunc(ctx, labID)
}

func (q *Querier) ListScheduleBlockingsForDateRange(ctx context.Context, arg db.ListScheduleBlockingsForDateRangeParams) ([]db.ScheduleBlocking, error) {
	if q.ListScheduleBlockingsForDateRangeFunc == nil {
		panic("dbfake: ListScheduleBlockingsForDateRange not implemented")
	}
	return q.ListScheduleBlockingsForDateRangeFunc(ctx, arg)
}

func (q *Querier) DeactivateScheduleBlocking(ctx context.Context, id int64) error {
	if q.DeactivateScheduleBlockingFunc == nil {
		panic("dbfake: DeactivateScheduleBlocking not implemented")
	}
	return q.DeactivateScheduleBlockingFunc(ctx, id)
}

func (q *Querier) CreateAppointment(ctx context.Context, arg db.CreateAppointmentParams) (db.Appointment, error) {
	if q.CreateAppointmentFunc == nil {
		panic("dbfake: CreateAppointment not implemented")
	}
	return q.CreateAppointmentFunc(ctx, arg)
}

func (q *Querier) GetAppointmentByID(ctx context.Context, id int64) (db.Appointment, error) {
	if q.GetAppointmentByIDFunc == nil {
		panic("dbfake: GetAppointmentByID not implemented")
	}
	return q.GetAppointmentByIDFunc(ctx, id)
}

func (q *Querier) GetAppointmentLabID(ctx context.Context, id int64) (int64, error) {
	if q.GetAppointmentLabIDFunc == nil {
		panic("dbfake: GetAppointmentLabID not implemented")
	}
	return q.GetAppointmentLabIDFunc(ctx, id)
}

func (q *Querier) ListAppointmentsByExperiment(ctx context.Context, arg db.ListAppointmentsByExperimentParams) ([]db.Appointment, error) {
	if q.ListAppointmentsByExperimentFunc == nil {
		panic("dbfake: ListAppointmentsByExperiment not implemented")
	}
	return q.ListAppointmentsByExperimentFunc(ctx, arg)
}

func (q *Querier) ScheduleAppointment(ctx context.Context, arg db.ScheduleAppointmentParams) (db.Appointment, error) {
	if q.ScheduleAppointmentFunc == nil {
		panic("dbfake: ScheduleAppointment not implemented")
	}
	return q.ScheduleAppointmentFunc(ctx, arg)
}

func (q *Querier) ReleaseAppointment(ctx context.Context, id int64) (db.Appointment, error) {
	if q.ReleaseAppointmentFunc == nil {
		panic("dbfake: ReleaseAppointment not implemented")
	}
	return q.ReleaseAppointmentFunc(ctx, id)
}

func (q *Querier) CreateAppointmentExperimenter(ctx context.Context, arg db.CreateAppointmentExperimenterParams) (db.AppointmentExperimenter, error) {
	if q.CreateAppointmentExperimenterFunc == nil {
		panic("dbfake: CreateAppointmentExperimenter not implemented")
	}
	return q.CreateAppointmentExperimenterFunc(ctx, arg)
}

func (q *Querier) ListAppointmentExperimenters(ctx context.Context, appointmentID int64) ([]db.AppointmentExperimenter, error) {
	if q.ListAppointmentExperimentersFunc == nil {
		panic("dbfake: ListAppointmentExperimenters not implemented")
	}
	return q.ListAppointmentExperimentersFunc(ctx, appointmentID)
}

func (q *Querier) ListBusyAppointmentExperimentersForDateRange(ctx context.Context, arg db.ListBusyAppointmentExperimentersForDateRangeParams) ([]db.ListBusyAppointmentExperimentersForDateRangeRow, error) {
	if q.ListBusyAppointmentExperimentersForDateRangeFunc == nil {
		panic("dbfake: ListBusyAppointmentExperimentersForDateRange not implemented")
	}
	return q.ListBusyAppointmentExperimentersForDateRangeFunc(ctx, arg)
}

func (q *Querier) ListBusyEquipmentForDateRange(ctx context.Context, arg db.ListBusyEquipmentForDateRangeParams) ([]db.ListBusyEquipmentForDateRangeRow, error) {
	if q.ListBusyEquipmentForDateRangeFunc == nil {
		panic("dbfake: ListBusyEquipmentForDateRange not implemented")
	}
	return q.ListBusyEquipmentForDateRangeFunc(ctx, arg)
}

func (q *Querier) ListEligibleChildrenForExperiment(ctx context.Context, arg db.ListEligibleChildrenForExperimentParams) ([]db.Child, error) {
	if q.ListEligibleChildrenForExperimentFunc == nil {
		panic("dbfake: ListEligibleChildrenForExperiment not implemented")
	}
	return q.ListEligibleChildrenForExperimentFunc(ctx, arg)
}

func (q *Querier) GetJobLastRun(ctx context.Context, jobName string) (pgtype.Timestamptz, error) {
	if q.GetJobLastRunFunc == nil {
		panic("dbfake: GetJobLastRun not implemented")
	}
	return q.GetJobLastRunFunc(ctx, jobName)
}

func (q *Querier) UpsertJobLastRun(ctx context.Context, jobName string) error {
	if q.UpsertJobLastRunFunc == nil {
		panic("dbfake: UpsertJobLastRun not implemented")
	}
	return q.UpsertJobLastRunFunc(ctx, jobName)
}

func (q *Querier) ListChangedAppointmentIDsSince(ctx context.Context, arg db.ListChangedAppointmentIDsSinceParams) ([]int64, error) {
	if q.ListChangedAppointmentIDsSinceFunc == nil {
		panic("dbfake: ListChangedAppointmentIDsSince not implemented")
	}
	return q.ListChangedAppointmentIDsSinceFunc(ctx, arg)
}

func (q *Querier) ListRecipientsForAppointments(ctx context.Context, appointmentIDs []int64) ([]db.ListRecipientsForAppointmentsRow, error) {
	if q.ListRecipientsForAppointmentsFunc == nil {
		panic("dbfake: ListRecipientsForAppointments not implemented")
	}
	return q.ListRecipientsForAppointmentsFunc(ctx, appointmentIDs)
}

func (q *Querier) ListPendingAppointmentsForUserInLab(ctx context.Context, arg db.ListPendingAppointmentsForUserInLabParams) ([]db.ListPendingAppointmentsForUserInLabRow, error) {
	if q.ListPendingAppointmentsForUserInLabFunc == nil {
		panic("dbfake: ListPendingAppointmentsForUserInLab not implemented")
	}
	return q.ListPendingAppointmentsForUserInLabFunc(ctx, arg)
}

func (q *Querier) ListAppointmentsDueForReminder(ctx context.Context, dueBefore pgtype.Timestamp) ([]db.ListAppointmentsDueForReminderRow, error) {
	if q.ListAppointmentsDueForReminderFunc == nil {
		panic("dbfake: ListAppointmentsDueForReminder not implemented")
	}
	return q.ListAppointmentsDueForReminderFunc(ctx, dueBefore)
}

func (q *Querier) MarkAppointmentReminderSent(ctx context.Context, id int64) error {
	if q.MarkAppointmentReminderSentFunc == nil {
		panic("dbfake: MarkAppointmentReminderSent not implemented")
	}
	return q.MarkAppointmentReminderSentFunc(ctx, id)
}
