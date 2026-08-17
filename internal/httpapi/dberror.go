package httpapi

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres SQLSTATE codes this file translates into 4xx responses. Anything
// else is treated as unexpected: logged and reported as a 500.
const (
	pgCheckViolation      = "23514"
	pgForeignKeyViolation = "23503"
	pgUniqueViolation     = "23505"
)

// writeDBError inspects err for a recognized Postgres error and writes the
// appropriate HTTP response, logging anything it doesn't recognize as a
// 500 rather than exposing the raw database error to the client.
func (s *Server) writeDBError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgCheckViolation:
			writeError(w, http.StatusBadRequest, "invalid value")
			return
		case pgForeignKeyViolation:
			writeError(w, http.StatusBadRequest, "referenced record does not exist")
			return
		case pgUniqueViolation:
			writeError(w, http.StatusConflict, "already exists")
			return
		}
	}

	s.logger.Error("database error", "error", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
