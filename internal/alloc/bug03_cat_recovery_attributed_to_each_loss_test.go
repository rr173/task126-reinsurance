package alloc

import (
	"context"
	"testing"
	"time"

	"task126-reinsurance/internal/contract"
	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/loss"
	"task126-reinsurance/internal/policy"
	"task126-reinsurance/internal/report"
	"task126-reinsurance/internal/store"
)

func TestBug03_CatRecoveryIsAttributedToEveryEventLoss(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, ":memory:")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = s.Close() })
	cs := contract.New(s)
	c, err := cs.Create(ctx, &domain.Contract{Code: "CAT-ATTR", Type: domain.CatXL, AttachmentPoint: 500000, Limit: 2000000, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)})
	if err != nil { t.Fatal(err) }
	ps := policy.New(s)
	createLoss := func(no string, paid domain.Money) *domain.Loss {
		p, err := ps.Create(ctx, &domain.Policy{PolicyNo: no + "-P", ContractID: c.ID, SumInsured: 5000000, OriginalPremium: 100000, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)})
		if err != nil { t.Fatal(err) }
		l, err := loss.New(s).Create(ctx, &domain.Loss{ClaimNo: no, PolicyID: p.ID, PaidAmount: paid, EventTag: "TYPHOON-9", OccurrenceDate: time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)})
		if err != nil { t.Fatal(err) }
		return l
	}
	first := createLoss("CAT-ATTR-1", 500000)
	second := createLoss("CAT-ATTR-2", 1500000)
	eng := New(s)
	if _, err := eng.AllocateEvent(ctx, c.ID, "TYPHOON-9"); err != nil { t.Fatal(err) }
	reports := report.New(s)
	for _, check := range []struct { id int64; want domain.Money }{{first.ID, 375000}, {second.ID, 1125000}} {
		r, err := reports.Loss(ctx, check.id)
		if err != nil { t.Fatal(err) }
		if got := r.Loss.PaidAmount.Sub(r.NetRetained); got != check.want {
			t.Fatalf("loss %d recovered=%d want=%d; every event loss must carry its own attributable cat recovery", check.id, got, check.want)
		}
	}
}
