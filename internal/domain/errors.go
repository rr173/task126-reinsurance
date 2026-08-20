package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors for the reinsurance engine. Handlers map these to HTTP status
// codes via httpapi.errorCodeFor.
var (
	// ErrInvalidArgument is returned for malformed input or business rule
	// violations that are the caller's fault (4xx).
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrNotFound is returned when a referenced entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned for unique-constraint or state conflicts.
	ErrConflict = errors.New("conflict")
	// ErrInvariant is returned when an operation violates a domain invariant.
	ErrInvariant = errors.New("invariant")
)

// business error wraps a sentinel with a human message.
type businessError struct {
	sentinel error
	msg      string
}

func (e *businessError) Error() string { return e.msg }
func (e *businessError) Unwrap() error { return e.sentinel }

// Newf wraps a sentinel error with a formatted message.
func Newf(sentinel error, format string, args ...any) error {
	return &businessError{sentinel: sentinel, msg: fmt.Sprintf(format, args...)}
}

// Is reports whether err matches the sentinel (supports errors.Is chains).
func Is(err, sentinel error) bool { return errors.Is(err, sentinel) }
