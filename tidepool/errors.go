package tidepool

import "errors"

// TidepoolError is the base error type.
type TidepoolError struct {
	Message    string
	StatusCode int
	Response   []byte
}

func (e *TidepoolError) Error() string {
	return e.Message
}

// Sentinel errors for type checking.
var (
	ErrValidation         = errors.New("validation error")
	ErrNotFound           = errors.New("not found")
	ErrServiceUnavailable = errors.New("service unavailable")
)

// IsValidationError checks if err is a validation error.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrValidation)
}

// IsNotFoundError checks if err is a not found error.
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsServiceUnavailableError checks if err is a service unavailable error.
func IsServiceUnavailableError(err error) bool {
	return errors.Is(err, ErrServiceUnavailable)
}
