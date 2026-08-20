package report

import (
	"context"
	"testing"
	"time"

	"task126-reinsurance/internal/alloc"
	"task126-reinsurance/internal/bordereaux"
	"task126-reinsurance/internal/contract"
	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/loss"
	"task126-reinsurance/internal/policy"
	"task126-reinsurance/internal/store"
)
func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
// TestContractReportAggregation drives a full proportional loop and verifies
// the report aggregates policy count, loss count and recovered totals.
func TestContractReportAggregation(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	cs := contract.New(s)
	c, err := cs.Create(ctx, &domain.Contract{
		Code: "REP1", Type: domain.QuotaShare, CessionRate: 0.5, CedingCommissionRate: 0.1,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ps := policy.New(s)
	p, err := ps.Create(ctx, &domain.Policy{
		PolicyNo: "PREP1", ContractID: c.ID, SumInsured: 1000000, OriginalPremium: 400000,
		// Policy written in Q2 so ceded premium lands in the Q2 bordereaux.
		StartDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ls := loss.New(s)
	l, err := ls.Create(ctx, &domain.Loss{
		ClaimNo: "LREP1", PolicyID: p.ID, PaidAmount: 1000000,
		OccurrenceDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	eng := alloc.New(s)
	if _, err := eng.AllocateLoss(ctx, l.ID); err != nil {
		t.Fatal(err)
	}
	bs := bordereaux.New(s)
	b, err := bs.Ensure(ctx, c.ID, 2026, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bs.Settle(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	rsvc := New(s)
	rep, err := rsvc.Contract(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rep.PolicyCount != 1 || rep.LossCount != 1 {
		t.Fatalf("counts: %+v", rep)
	}
	if rep.TotalRecovered != 500000 {
		t.Errorf("recovered: got %d want 500000", int64(rep.TotalRecovered))
	}
	if rep.BordereauxCount != 1 {
		t.Errorf("bord count: %d", rep.BordereauxCount)
	}
	// Net balance from the settled bordereaux (2000 - 5000 - 200 = -3200).
	if rep.NetBalance != -320000 {
		t.Errorf("net balance: got %d want -320000", int64(rep.NetBalance))
	}
}
// TestLossesAllReport verifies the loss report includes allocations and net
// retained.
func TestLossesAllReport(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	cs := contract.New(s)
	c, err := cs.Create(ctx, &domain.Contract{
		Code: "REP2", Type: domain.PerRiskXL, AttachmentPoint: 500000, Limit: 1000000,
		NumReinstatements: 0, ReinstatementFactor: 1, CededPremiumRate: 1,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ps := policy.New(s)
	p, err := ps.Create(ctx, &domain.Policy{
		PolicyNo: "PREP2", ContractID: c.ID, SumInsured: 10000000, OriginalPremium: 200000,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ls := loss.New(s)
	l, err := ls.Create(ctx, &domain.Loss{
		ClaimNo: "Lrep2", PolicyID: p.ID, PaidAmount: 800000,
		OccurrenceDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	eng := alloc.New(s)
	if _, err := eng.AllocateLoss(ctx, l.ID); err != nil {
		t.Fatal(err)
	}
	rsvc := New(s)
	reps, err := rsvc.LossesAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(reps) != 1 {
		t.Fatalf("expected 1 loss report, got %d", len(reps))
	}
	// Loss 8000.00 (in cents 800000), attachment 5000.00 (500000) -> recovery 3000.00 (300000).
	// Net retained = 800000 - 300000 = 500000.
	if reps[0].NetRetained != 500000 {
		t.Errorf("net retained: got %d want 500000", int64(reps[0].NetRetained))
	}
}
