// Package dbfake provides a fake implementation of db.Querier for tests,
// so packages that depend on db.Querier can be tested without a real
// database connection.
package dbfake

import (
	"context"

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

	CreateFamilyFunc  func(ctx context.Context, arg db.CreateFamilyParams) (db.Family, error)
	GetFamilyByIDFunc func(ctx context.Context, id int64) (db.Family, error)
	UpdateFamilyFunc  func(ctx context.Context, arg db.UpdateFamilyParams) (db.Family, error)

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

	CreateNoteFunc        func(ctx context.Context, arg db.CreateNoteParams) (db.Note, error)
	ListNotesByEntityFunc func(ctx context.Context, arg db.ListNotesByEntityParams) ([]db.Note, error)

	ListActiveRecruitmentSourcesFunc func(ctx context.Context) ([]db.RecruitmentSource, error)
	CreateRecruitmentSourceFunc      func(ctx context.Context, name string) (db.RecruitmentSource, error)
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
