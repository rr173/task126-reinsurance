package loss

import (
	"context"
	"testing"
	"time"

	"task126-reinsurance/internal/alloc"
	"task126-reinsurance/internal/contract"
	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/policy"
)

func TestBug02_SealedCatEventRejectsLateLoss(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	c, err := contract.New(s).Create(ctx, &domain.Contract{Code: "CAT-SEAL", Type: domain.CatXL, AttachmentPoint: 50_000, Limit: 100_000, StartDate: start, EndDate: end})
	if err != nil { t.Fatal(err) }
	p, err := policy.New(s).Create(ctx, &domain.Policy{PolicyNo: "P-SEAL", ContractID: c.ID, SumInsured: 1_000_000, OriginalPremium: 100_000, StartDate: start, EndDate: end})
	if err != nil { t.Fatal(err) }
	service := New(s)
	for i, amount := range []domain.Money{30_000, 40_000} {
		_, err := service.Create(ctx, &domain.Loss{ClaimNo: string(rune('A'+i)), PolicyID: p.ID, PaidAmount: amount, EventTag: "storm-sealed", OccurrenceDate: time.Date(2026, 6, i+1, 0, 0, 0, 0, time.UTC)})
		if err != nil { t.Fatal(err) }
	}
	if _, err := alloc.New(s).AllocateEvent(ctx, c.ID, "storm-sealed"); err != nil { t.Fatal(err) }
	_, err = service.Create(ctx, &domain.Loss{ClaimNo: "LATE-SEAL", PolicyID: p.ID, PaidAmount: 10_000, EventTag: "storm-sealed", OccurrenceDate: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)})
	if !domain.Is(err, domain.ErrConflict) { t.Fatalf("late loss for a sealed catastrophe event should conflict, got %v", err) }
}
