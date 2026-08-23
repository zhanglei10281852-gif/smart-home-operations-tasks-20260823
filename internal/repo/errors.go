package repo

import (
	"database/sql"
	"errors"

	"github.com/lib/pq"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type classifiedError struct {
	public error
	cause  error
}

func (e classifiedError) Error() string   { return e.public.Error() }
func (e classifiedError) Unwrap() []error { return []error{e.public, e.cause} }

func classifyReadError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return classifiedError{public: model.ErrNotFound, cause: err}
	}
	return classifyWriteError(err)
}

func classifyWriteError(err error) error {
	if err == nil {
		return nil
	}
	var postgres *pq.Error
	if !errors.As(err, &postgres) {
		return err
	}
	var public error
	switch postgres.Code {
	case "23505", "23503", "40001", "40P01":
		public = model.ErrConflict
	case "23502", "23514", "22001", "22P02":
		public = model.ErrInvalid
	default:
		return err
	}
	return classifiedError{public: public, cause: err}
}

func isRetryableSerializable(err error) bool {
	var postgres *pq.Error
	if !errors.As(err, &postgres) {
		return false
	}
	return postgres.Code == "40001" || postgres.Code == "40P01"
}
