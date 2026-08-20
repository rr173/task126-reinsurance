package loss

import (
	"context"
	"testing"
	"time"

	"task126-reinsurance/internal/contract"
	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/policy"
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

// TestCatLossRequiresEventTag verifies cat_xl losses must carry an event_tag.
func TestCatLossRequiresEventTag(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	cs := contract.New(s)
	c, err := cs.Create(ctx, &domain.Contract{
		Code: "CAT", Type: domain.CatXL, AttachmentPoint: 500000, Limit: 1000000,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ps := policy.New(s)
	p, err := ps.Create(ctx, &domain.Policy{
		PolicyNo: "PC", ContractID: c.ID, SumInsured: 10000000, OriginalPremium: 500000,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ls := New(s)
	_, err = ls.Create(ctx, &domain.Loss{
		ClaimNo: "L1", PolicyID: p.ID, PaidAmount: 300000,
		OccurrenceDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		// EventTag intentionally empty.
	})
	if !domain.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("cat loss without event_tag should be rejected, got %v", err)
	}
}

// TestLossOutsidePolicyWindowRejected verifies the occurrence date window.
func TestLossOutsidePolicyWindowRejected(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	cs := contract.New(s)
	c, err := cs.Create(ctx, &domain.Contract{
		Code: "QS", Type: domain.QuotaShare, CessionRate: 0.5,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ps := policy.New(s)
	p, err := ps.Create(ctx, &domain.Policy{
		PolicyNo: "P1", ContractID: c.ID, SumInsured: 1000000, OriginalPremium: 100000,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ls := New(s)
	_, err = ls.Create(ctx, &domain.Loss{
		ClaimNo: "LO", PolicyID: p.ID, PaidAmount: 100000,
		OccurrenceDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), // after policy end
	})
	if !domain.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("out-of-window loss should be rejected, got %v", err)
	}
}
