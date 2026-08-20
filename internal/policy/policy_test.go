package policy

import (
	"context"
	"testing"
	"time"

	"task126-reinsurance/internal/contract"
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

// TestSurplusCessionDerived verifies the cession rate is computed at policy
// registration from the sum insured.
func TestSurplusCessionDerived(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	cs := contract.New(s)
	c, err := cs.Create(ctx, &domain.Contract{
		Code: "SS", Type: domain.SurplusShare,
		RetainedLine: 2000000, TreatyCapacity: 3,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ps := New(s)
	p, err := ps.Create(ctx, &domain.Policy{
		PolicyNo: "P1", ContractID: c.ID, SumInsured: 8000000, OriginalPremium: 400000,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if diff := p.CessionRate - 0.75; diff < -1e-9 || diff > 1e-9 {
		t.Errorf("cession rate: got %v want 0.75", p.CessionRate)
	}
}

// TestPolicyOutsideContractWindowRejected verifies the date-window guard.
func TestPolicyOutsideContractWindowRejected(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	cs := contract.New(s)
	c, err := cs.Create(ctx, &domain.Contract{
		Code: "QSW", Type: domain.QuotaShare, CessionRate: 0.5,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ps := New(s)
	// Policy end beyond contract end -> rejected.
	_, err = ps.Create(ctx, &domain.Policy{
		PolicyNo: "PO", ContractID: c.ID, SumInsured: 1000000, OriginalPremium: 100000,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if !domain.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("out-of-window policy should be rejected, got %v", err)
	}
}
