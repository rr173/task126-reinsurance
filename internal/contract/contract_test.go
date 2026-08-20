package contract

import (
	"context"
	"testing"
	"time"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestCreateValidatesType checks that type-specific fields are enforced.
func TestCreateValidatesType(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	svc := New(s)
	// quota_share with cession_rate=0 -> rejected.
	_, err := svc.Create(ctx, &domain.Contract{
		Code: "BAD", Type: domain.QuotaShare, CessionRate: 0,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if !domain.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

// TestTransitionStatus checks allowed and disallowed transitions.
func TestTransitionStatus(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	svc := New(s)
	c, err := svc.Create(ctx, &domain.Contract{
		Code: "TR", Type: domain.QuotaShare, CessionRate: 0.5,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TransitionStatus(ctx, c.ID, domain.ContractExpired); err != nil {
		t.Fatalf("in_force -> expired should succeed: %v", err)
	}
	// expired -> in_force is not allowed.
	_, err = svc.TransitionStatus(ctx, c.ID, domain.ContractInForce)
	if !domain.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("expired -> in_force should be rejected, got %v", err)
	}
}

// TestTransitionToInvalidStatus checks unknown status rejection.
func TestTransitionToInvalidStatus(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	svc := New(s)
	c, err := svc.Create(ctx, &domain.Contract{
		Code: "INV", Type: domain.QuotaShare, CessionRate: 0.5,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.TransitionStatus(ctx, c.ID, domain.ContractStatus("bogus"))
	if !domain.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("invalid status should be rejected, got %v", err)
	}
}
