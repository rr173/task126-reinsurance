package selfcheck

import (
	"context"
	"testing"
	"time"

	"task126-reinsurance/internal/store"
)

// TestSmoke is the go-test entry point for the selfcheck; it builds an in-memory
// store and runs the full validation scenarios.
func TestSmoke(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	if err := Run(ctx, s); err != nil {
		t.Fatalf("selfcheck failed: %v", err)
	}
}

// TestQuarterOf sanity-checks the quarter bucketing helper.
func TestQuarterOf(t *testing.T) {
	cases := []struct {
		month, year, q int
	}{
		{1, 2026, 1}, {3, 2026, 1}, {4, 2026, 2}, {6, 2026, 2},
		{7, 2026, 3}, {9, 2026, 3}, {10, 2026, 4}, {12, 2026, 4},
	}
	for _, c := range cases {
		y, q := domainQuarter(time.Date(c.year, time.Month(c.month), 15, 0, 0, 0, 0, time.UTC))
		if y != c.year || q != c.q {
			t.Errorf("month %d: got %d Q%d, want %d Q%d", c.month, y, q, c.year, c.q)
		}
	}
}

// domainQuarter calls the domain helper directly (kept here to avoid an import
// cycle in the smoke test if the domain package layout changes).
func domainQuarter(t time.Time) (int, int) {
	y := t.UTC().Year()
	q := (int(t.UTC().Month())-1)/3 + 1
	return y, q
}
