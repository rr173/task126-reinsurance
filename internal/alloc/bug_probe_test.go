package alloc

import (
	"context"
	"testing"
	"time"

	"task126-reinsurance/internal/contract"
	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/loss"
	"task126-reinsurance/internal/policy"
	"task126-reinsurance/internal/store"
)

func TestBug01_HistoricalLossRemainsRecoverableAfterExpiry(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, ":memory:")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { s.Close() })
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	c, err := contract.New(s).Create(ctx, &domain.Contract{Code: "QS-HIST", Type: domain.QuotaShare, CessionRate: 0.5, StartDate: start, EndDate: end})
	if err != nil { t.Fatal(err) }
	p, err := policy.New(s).Create(ctx, &domain.Policy{PolicyNo: "P-HIST", ContractID: c.ID, SumInsured: 1_000_000, OriginalPremium: 100_000, StartDate: start, EndDate: end})
	if err != nil { t.Fatal(err) }
	l, err := loss.New(s).Create(ctx, &domain.Loss{ClaimNo: "L-HIST", PolicyID: p.ID, PaidAmount: 80_000, OccurrenceDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil { t.Fatal(err) }
	if _, err := contract.New(s).TransitionStatus(ctx, c.ID, domain.ContractExpired); err != nil { t.Fatal(err) }
	allocs, err := New(s).AllocateLoss(ctx, l.ID)
	if err != nil { t.Fatalf("historical loss should remain recoverable after expiry: %v", err) }
	if len(allocs) != 1 || allocs[0].RecoveredAmount != 40_000 { t.Fatalf("recovery=%v, want one layer of 40000", allocs) }
}
