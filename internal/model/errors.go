package model

import "fmt"

const (
	ErrCodeInvalidArgument = "INVALID_ARGUMENT"
)

type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func invalid(msg string) *ValidationError {
	return &ValidationError{Code: ErrCodeInvalidArgument, Message: msg}
}

func invalidf(format string, args ...any) *ValidationError {
	return invalid(fmt.Sprintf(format, args...))
}
