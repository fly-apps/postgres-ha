package render

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

func JSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

type errRes struct {
	Error string `json:"error"`
}

// Error should be able to render custom error from pkg/flypg/admin/error.go:
// package admin
// it will use errors.As to check if the error is an admin
func Err(w http.ResponseWriter, err error) {
	JSON(w, errRes{Error: err.Error()}, status(err))
}

func status(err error) int {
	if err == nil {
		return http.StatusOK
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound
	}

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		// fmt.Printf("%s: %s\n", pgErr.Code, pgErr.Message)
		switch pgErr.Code {
		case "42710": // unique violation
			return http.StatusConflict
		case "23505": // unique violation
			return http.StatusConflict
		case "23503": // foreign key violation
			return http.StatusBadRequest
		case "23502": // not null violation
			return http.StatusBadRequest
		default:
			return http.StatusInternalServerError
		}
	}
	return http.StatusInternalServerError
}
