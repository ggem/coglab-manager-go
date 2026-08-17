package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestWriteDBError(t *testing.T) {
	s := &Server{logger: discardLogger()}

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"no rows", pgx.ErrNoRows, http.StatusNotFound},
		{"check violation", &pgconn.PgError{Code: pgCheckViolation}, http.StatusBadRequest},
		{"foreign key violation", &pgconn.PgError{Code: pgForeignKeyViolation}, http.StatusBadRequest},
		{"unique violation", &pgconn.PgError{Code: pgUniqueViolation}, http.StatusConflict},
		{"unrecognized pg error", &pgconn.PgError{Code: "58000"}, http.StatusInternalServerError},
		{"unexpected error", assertErr("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.writeDBError(rec, tt.err)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
