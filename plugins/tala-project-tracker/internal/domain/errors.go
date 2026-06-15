package domain

import "fmt"

type ErrorCode string

const (
	CodeValidationError ErrorCode = "validation_error"
	CodeNotFound        ErrorCode = "not_found"
	CodeCycleDetected   ErrorCode = "cycle_detected"
	CodeMissingUsername ErrorCode = "missing_username"
	CodeConflict        ErrorCode = "conflict"
	CodeInternal        ErrorCode = "internal_error"
)

type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Field   string    `json:"field,omitempty"`
}

func (e *AppError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Field)
}

func NewError(code ErrorCode, message, field string) *AppError {
	return &AppError{Code: code, Message: message, Field: field}
}
