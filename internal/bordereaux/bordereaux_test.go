package bordereaux

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

// TestSettleReopenOrdering verifies the "no later settled quarter" guard on
// reopening a settled bordereaux.
func TestSettleReopenOrdering(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	svc := New(s)
	cs := newContract(ctx, t, s, "QS-ORD")

	// Settle Q1 and Q2.
	b1, err := svc.Ensure(ctx, cs.ID, 2026, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Settle(ctx, b1.ID); err != nil {
		t.Fatal(err)
	}
	b2, err := svc.Ensure(ctx, cs.ID, 2026, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Settle(ctx, b2.ID); err != nil {
		t.Fatal(err)
	}
	// Reopen Q1 while Q2 settled -> conflict.
	_, err = svc.Reopen(ctx, b1.ID)
	if !domain.Is(err, domain.ErrConflict) {
		t.Fatalf("reopen earlier while later settled should conflict, got %v", err)
	}
	// Reopen Q2 (latest) -> ok.
	if _, err := svc.Reopen(ctx, b2.ID); err != nil {
		t.Fatalf("reopen latest: %v", err)
	}
}

// TestSettleTwiceRejected verifies a settled bordereaux cannot be settled again.
func TestSettleTwiceRejected(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	svc := New(s)
	c := newContract(ctx, t, s, "QS-2X")
	b, err := svc.Ensure(ctx, c.ID, 2026, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Settle(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Settle(ctx, b.ID)
	if !domain.Is(err, domain.ErrConflict) {
		t.Fatalf("second settle should conflict, got %v", err)
	}
}

// TestEnsureIdempotent verifies the upsert is idempotent per (contract, quarter).
func TestEnsureIdempotent(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	svc := New(s)
	c := newContract(ctx, t, s, "QS-ID")
	b1, err := svc.Ensure(ctx, c.ID, 2026, 3)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := svc.Ensure(ctx, c.ID, 2026, 3)
	if err != nil {
		t.Fatal(err)
	}
	if b1.ID != b2.ID {
		t.Fatalf("ensure not idempotent: %d vs %d", b1.ID, b2.ID)
	}
}

func newContract(ctx context.Context, t *testing.T, s *store.Store, code string) *domain.Contract {
	t.Helper()
	cs := contract.New(s)
	c := &domain.Contract{
		Code: code, Type: domain.QuotaShare, CessionRate: 0.5,
		CedingCommissionRate: 0, BrokerRate: 0, CededPremiumRate: 1,
		Currency: "CNY",
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	if _, err := cs.Create(ctx, c); err != nil {
		t.Fatalf("create contract: %v", err)
	}
	return c
}