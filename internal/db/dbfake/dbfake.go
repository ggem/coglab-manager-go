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
