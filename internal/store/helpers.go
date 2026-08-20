package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"task126-reinsurance/internal/domain"
)

// ts formats a time as RFC3339Nano UTC for storage.
func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// parseTs parses an RFC3339Nano timestamp.
func parseTs(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// contractDate formats a date as YYYY-MM-DD.
func contractDate(t time.Time) string { return t.UTC().Format("2006-01-02") }

// parseDate parses a YYYY-MM-DD into a UTC midnight time.
func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// mapErr translates database errors into domain sentinels where possible.
func mapErr(err error, what string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Newf(domain.ErrNotFound, "%s: not found", what)
	}
	msg := err.Error()
	if strings.Contains(msg, "UNIQUE") || strings.Contains(msg, "unique") {
		return domain.Newf(domain.ErrConflict, "%s: duplicate", what)
	}
	return domain.Newf(domain.ErrInvalidArgument, "%s: %v", what, err)
}
