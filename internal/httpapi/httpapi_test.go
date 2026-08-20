package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

func openAPI(t *testing.T) (*API, *store.Store) {
	t.Helper()
	s, err := store.Open(t.Context(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	api, err := New(s)
	if err != nil {
		t.Fatalf("new api: %v", err)
	}
	return api, s
}

// TestCreateContractHTTPRoundTrip exercises the full HTTP create->list path.
func TestCreateContractHTTPRoundTrip(t *testing.T) {
	api, _ := openAPI(t)
	h := api.Router()
	body := `{"code":"HTTP1","type":"quota_share","cession_rate":0.5,"start_date":"2026-01-01","end_date":"2026-12-31"}`
	req := httptest.NewRequest(http.MethodPost, "/contracts", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status: got %d want %d, body %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var c contractDTO
	_ = json.NewDecoder(w.Body).Decode(&c)
	if c.Code != "HTTP1" || c.ID == 0 {
		t.Fatalf("create response: %+v", c)
	}
	// List.
	req = httptest.NewRequest(http.MethodGet, "/contracts", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status: %d", w.Code)
	}
	var list []contractDTO
	_ = json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 1 || list[0].Code != "HTTP1" {
		t.Fatalf("list: %+v", list)
	}
}

// TestCreateContractValidationHTTP verifies 4xx on bad input.
func TestCreateContractValidationHTTP(t *testing.T) {
	api, _ := openAPI(t)
	h := api.Router()
	// cession_rate out of range -> 400.
	body := `{"code":"BAD","type":"quota_share","cession_rate":1.5,"start_date":"2026-01-01","end_date":"2026-12-31"}`
	req := httptest.NewRequest(http.MethodPost, "/contracts", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body %s", w.Code, w.Body.String())
	}
}

// TestHealthz verifies the health endpoint.
func TestHealthz(t *testing.T) {
	api, _ := openAPI(t)
	h := api.Router()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz: got %d", w.Code)
	}
	var m map[string]string
	_ = json.NewDecoder(w.Body).Decode(&m)
	if m["status"] != "ok" {
		t.Fatalf("healthz body: %v", m)
	}
}

// TestErrorCodeMapping verifies domain errors map to HTTP codes.
func TestErrorCodeMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{domain.Newf(domain.ErrInvalidArgument, "x"), http.StatusBadRequest},
		{domain.Newf(domain.ErrNotFound, "x"), http.StatusNotFound},
		{domain.Newf(domain.ErrConflict, "x"), http.StatusConflict},
		{domain.Newf(domain.ErrInvariant, "x"), http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		_, status := errorCodeFor(tc.err)
		if status != tc.status {
			t.Errorf("status for %v: got %d want %d", tc.err, status, tc.status)
		}
	}
}
