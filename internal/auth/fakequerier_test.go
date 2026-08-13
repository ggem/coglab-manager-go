package auth

import (
	"context"

	"github.com/ggem/coglab-manager-go/internal/db"
)

// fakeQuerier implements db.Querier for tests, with each method backed by
// an optional function field. Calling a method whose field is unset panics
// with a clear message, so a test exercising a code path it didn't expect
// fails loudly instead of returning a silent zero value.
type fakeQuerier struct {
	createUser            func(ctx context.Context, arg db.CreateUserParams) (db.User, error)
	getUserByEmail        func(ctx context.Context, email string) (db.User, error)
	getUserByID           func(ctx context.Context, id int64) (db.User, error)
	createSession         func(ctx context.Context, arg db.CreateSessionParams) (db.Session, error)
	getSessionByTokenHash func(ctx context.Context, tokenHash []byte) (db.Session, error)
	revokeSession         func(ctx context.Context, tokenHash []byte) error
	touchSessionLastSeen  func(ctx context.Context, id int64) error
}

func (f *fakeQuerier) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	if f.createUser == nil {
		panic("fakeQuerier: CreateUser not implemented")
	}
	return f.createUser(ctx, arg)
}

func (f *fakeQuerier) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	if f.getUserByEmail == nil {
		panic("fakeQuerier: GetUserByEmail not implemented")
	}
	return f.getUserByEmail(ctx, email)
}

func (f *fakeQuerier) GetUserByID(ctx context.Context, id int64) (db.User, error) {
	if f.getUserByID == nil {
		panic("fakeQuerier: GetUserByID not implemented")
	}
	return f.getUserByID(ctx, id)
}

func (f *fakeQuerier) CreateSession(ctx context.Context, arg db.CreateSessionParams) (db.Session, error) {
	if f.createSession == nil {
		panic("fakeQuerier: CreateSession not implemented")
	}
	return f.createSession(ctx, arg)
}

func (f *fakeQuerier) GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (db.Session, error) {
	if f.getSessionByTokenHash == nil {
		panic("fakeQuerier: GetSessionByTokenHash not implemented")
	}
	return f.getSessionByTokenHash(ctx, tokenHash)
}

func (f *fakeQuerier) RevokeSession(ctx context.Context, tokenHash []byte) error {
	if f.revokeSession == nil {
		panic("fakeQuerier: RevokeSession not implemented")
	}
	return f.revokeSession(ctx, tokenHash)
}

func (f *fakeQuerier) TouchSessionLastSeen(ctx context.Context, id int64) error {
	if f.touchSessionLastSeen == nil {
		panic("fakeQuerier: TouchSessionLastSeen not implemented")
	}
	return f.touchSessionLastSeen(ctx, id)
}
