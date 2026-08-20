package selfcheck

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

// Run exercises the full business loop against an in-memory store and asserts
// the locked validation scenarios. It returns a non-nil error if any check
// fails, with a message identifying the failing scenario.
//
// The selfcheck is a real end-to-end test: it creates contracts, policies and
// losses, allocates recoveries, triggers reinstatements, settles bordereaux
// and verifies restart recovery -- the same flows the HTTP API exposes.
func Run(ctx context.Context, s *store.Store) error {
	h := newHarness(s)

	// Scenario 1: proportional quota_share allocation.
	if err := h.scenarioProportional(ctx); err != nil {
		return fmt.Errorf("scenario proportional: %w", err)
	}
	// Scenario 2: surplus_share cession rate derivation.
	if err := h.scenarioSurplus(ctx); err != nil {
		return fmt.Errorf("scenario surplus: %w", err)
	}
	// Scenario 3: excess layered allocation + limit decrement.
	if err := h.scenarioExcess(ctx); err != nil {
		return fmt.Errorf("scenario excess: %w", err)
	}
	// Scenario 4: reinstatement premium.
	if err := h.scenarioReinstatement(ctx); err != nil {
		return fmt.Errorf("scenario reinstatement: %w", err)
	}
	// Scenario 5: catastrophe accumulation.
	if err := h.scenarioCatastrophe(ctx); err != nil {
		return fmt.Errorf("scenario catastrophe: %w", err)
	}
	// Scenario 6: bordereaux settle + lock.
	if err := h.scenarioBordereauxSettle(ctx); err != nil {
		return fmt.Errorf("scenario bordereaux settle: %w", err)
	}
	// Scenario 7: bordereaux reopen ordering.
	if err := h.scenarioBordereauxReopen(ctx); err != nil {
		return fmt.Errorf("scenario bordereaux reopen: %w", err)
	}
	// Scenario 8: contract not-in-force rejection.
	if err := h.scenarioContractExpiry(ctx); err != nil {
		return fmt.Errorf("scenario contract expiry: %w", err)
	}
	// Scenario 9: restart recovery on a fresh store re-opened from the same DSN.
	if err := h.scenarioRestart(ctx); err != nil {
		return fmt.Errorf("scenario restart: %w", err)
	}
	return nil
}

// helpers -----------------------------------------------------------------

func assert(cond bool, msg string, args ...any) error {
	if !cond {
		return fmt.Errorf(msg, args...)
	}
	return nil
}

func assertMoney(got, want domain.Money, label string) error {
	if got != want {
		return fmt.Errorf("%s: got %d cents, want %d", label, int64(got), int64(want))
	}
	return nil
}

func assertErrIs(err error, sentinel error, label string) error {
	if err == nil {
		return fmt.Errorf("%s: expected error, got nil", label)
	}
	if !errors.Is(err, sentinel) {
		return fmt.Errorf("%s: got %v, want %v", label, err, sentinel)
	}
	return nil
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }

func jan1(year int) time.Time { return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC) }
func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
