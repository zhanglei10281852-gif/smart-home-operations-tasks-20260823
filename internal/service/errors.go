package service

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrorValidation ErrorCode = "validation"
	ErrorConflict   ErrorCode = "conflict"
	ErrorDependency ErrorCode = "dependency"
	ErrorInternal   ErrorCode = "internal"
)

type AppError struct {
	Code ErrorCode
	Op   string
	Err  error
}

func (e *AppError) Error() string { return fmt.Sprintf("%s: %s: %v", e.Code, e.Op, e.Err) }
func (e *AppError) Unwrap() error { return e.Err }
func Wrap(code ErrorCode, op string, err error) error {
	if err == nil {
		return nil
	}
	return &AppError{Code: code, Op: op, Err: err}
}
func IsCode(err error, code ErrorCode) bool {
	var app *AppError
	return errors.As(err, &app) && app.Code == code
}
