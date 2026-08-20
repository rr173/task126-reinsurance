package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"task126-reinsurance/internal/domain"
)

// writeJSON marshals v as JSON with status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// writeError maps a domain error to an HTTP status and JSON body.
func writeError(w http.ResponseWriter, err error) {
	code, status := errorCodeFor(err)
	writeJSON(w, status, map[string]string{"error": err.Error(), "code": code})
}

// errorCodeFor returns a stable code string and HTTP status for a domain error.
func errorCodeFor(err error) (code string, status int) {
	switch {
	case domain.Is(err, domain.ErrNotFound):
		return "not_found", http.StatusNotFound
	case domain.Is(err, domain.ErrConflict):
		return "conflict", http.StatusConflict
	case domain.Is(err, domain.ErrInvariant):
		return "invariant", http.StatusUnprocessableEntity
	case domain.Is(err, domain.ErrInvalidArgument):
		return "invalid_argument", http.StatusBadRequest
	default:
		return "internal", http.StatusInternalServerError
	}
}

// decode reads JSON from r into dst. Returns a domain invalid-argument error on
// failure so handlers can propagate it uniformly.
func decode(r *http.Request, dst any) error {
	if r.Body == nil {
		return domain.Newf(domain.ErrInvalidArgument, "empty request body")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return domain.Newf(domain.ErrInvalidArgument, "invalid JSON: %v", err)
	}
	return nil
}

// moneyToCents exposes a Money as int64 cents for JSON.
func moneyToCents(m domain.Money) int64 { return int64(m) }

// centsToMoney parses a JSON integer cents value into Money.
func centsToMoney(c int64) domain.Money { return domain.Money(c) }

// pathID extracts the {id} path variable from a Go 1.22 ServeMux pattern.
func pathID(r *http.Request, name string) string {
	return r.PathValue(name)
}

// trimLower lowercases and trims a string.
func trimLower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
