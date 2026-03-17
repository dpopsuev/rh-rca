package rp

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError represents a structured error response from the Report Portal API.
// Callers should prefer the predicate functions (IsNotFound, IsUnauthorized, etc.)
// to inspect errors rather than asserting on this type directly.
type APIError struct {
	operation  string
	statusCode int
	errorCode  int
	message    string
}

func (e *APIError) Error() string {
	if e.errorCode != 0 {
		return fmt.Sprintf("%s: HTTP %d: [%d] %s", e.operation, e.statusCode, e.errorCode, e.message)
	}
	return fmt.Sprintf("%s: HTTP %d: %s", e.operation, e.statusCode, e.message)
}

func newAPIError(operation string, statusCode int, errorCode int, message string) *APIError {
	return &APIError{
		operation:  operation,
		statusCode: statusCode,
		errorCode:  errorCode,
		message:    message,
	}
}

// StatusCode returns the HTTP status code from the response.
func (e *APIError) StatusCode() int { return e.statusCode }

// ErrorCode returns the Report Portal application error code.
func (e *APIError) ErrorCode() int { return e.errorCode }

// Message returns the human-readable error message.
func (e *APIError) Message() string { return e.message }

// Operation returns a short description of the API call that failed.
func (e *APIError) Operation() string { return e.operation }

// IsNotFound reports whether err is an API error with HTTP 404 status.
func IsNotFound(err error) bool { return HasStatusCode(err, http.StatusNotFound) }

// IsUnauthorized reports whether err is an API error with HTTP 401 status.
func IsUnauthorized(err error) bool { return HasStatusCode(err, http.StatusUnauthorized) }

// IsForbidden reports whether err is an API error with HTTP 403 status.
func IsForbidden(err error) bool { return HasStatusCode(err, http.StatusForbidden) }

// HasStatusCode reports whether err is an API error whose HTTP status code matches.
func HasStatusCode(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.statusCode == code
}

// HasErrorCode reports whether err is an API error whose RP error code matches.
func HasErrorCode(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.errorCode == code
}
