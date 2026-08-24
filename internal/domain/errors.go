package domain

import "fmt"

type ErrorKind string

const (
	ErrorValidation ErrorKind = "validation"
	ErrorConflict   ErrorKind = "conflict"
	ErrorState      ErrorKind = "state"
	ErrorNotFound   ErrorKind = "not_found"
)

type BusinessError struct {
	Kind    ErrorKind      `json:"kind"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Field   string         `json:"field,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *BusinessError) Error() string { return e.Message }

func NewValidation(code, field, message string) error {
	return &BusinessError{Kind: ErrorValidation, Code: code, Field: field, Message: message}
}

func NewState(code, message string) error {
	return &BusinessError{Kind: ErrorState, Code: code, Message: message}
}

func NewConflict(code, message string) error {
	return &BusinessError{Kind: ErrorConflict, Code: code, Message: message}
}

func NewDetailedConflict(code, message string, details map[string]any) error {
	return &BusinessError{Kind: ErrorConflict, Code: code, Message: message, Details: details}
}

func RevisionConflict(expected, actual int) error {
	return NewConflict("revision_conflict", fmt.Sprintf("修订号冲突：期望 %d，当前为 %d", expected, actual))
}
