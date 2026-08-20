package alloc

import (
	"context"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

// CatastropheAccumulator accumulates losses for a (contract, event_tag) pair
// before a cat_xl layer applies. It ensures losses from the same catastrophe
// event are grouped and not double-recovered against the same contract.
type CatastropheAccumulator struct {
	losses store.LossStore
	events store.CatastropheEventStore
}

// NewCatastropheAccumulator returns an accumulator bound to store s.
func NewCatastropheAccumulator(s *store.Store) *CatastropheAccumulator {
	return &CatastropheAccumulator{
		losses: store.NewLossStore(),
		events: store.NewCatastropheEventStore(),
	}
}

// AccumulateFor adds lossAmount to the accumulated loss for (contractID,
// eventTag) within tx. It is idempotent against re-accumulation of an already
// marked-allocated event only if the caller checks first; raw accumulation is
// additive so a single loss must be added exactly once.
func (a *CatastropheAccumulator) AccumulateFor(ctx context.Context, tx store.DBTX, contractID int64, eventTag string, lossAmount domain.Money) error {
	if eventTag == "" {
		return domain.Newf(domain.ErrInvalidArgument, "cat_xl accumulation requires a non-empty event_tag")
	}
	return a.events.UpsertAccumulated(ctx, tx, contractID, eventTag, lossAmount)
}

// AccumulatedFor returns the accumulated loss and whether it was already
// allocated for a (contract, event_tag) pair.
func (a *CatastropheAccumulator) AccumulatedFor(ctx context.Context, tx store.DBTX, contractID int64, eventTag string) (domain.Money, bool, error) {
	e, err := a.events.Get(ctx, tx, contractID, eventTag)
	if err != nil {
		if domain.Is(err, domain.ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return e.AccumulatedLoss, e.Allocated, nil
}

// RequireOpen shares the event-seal invariant between event allocation and
// loss registration.
func (a *CatastropheAccumulator) RequireOpen(ctx context.Context, tx store.DBTX, contractID int64, eventTag string) error {
	return a.events.RequireOpen(ctx, tx, contractID, eventTag)
}

// MarkAllocated flags the event as allocated.
func (a *CatastropheAccumulator) MarkAllocated(ctx context.Context, tx store.DBTX, contractID int64, eventTag string) error {
	return a.events.MarkAllocated(ctx, tx, contractID, eventTag)
}

// Reconcile ensures the accumulated loss matches the sum of paid losses for the
// (contract, event_tag) pair; the paid-loss sum is authoritative. It also writes
// the authoritative accumulated loss back to the catastrophe_events row so the
// allocation step and MarkAllocated have a row to operate on.
func (a *CatastropheAccumulator) Reconcile(ctx context.Context, tx store.DBTX, contractID int64, eventTag string) (domain.Money, error) {
	sum, err := a.losses.SumPaidByContractEvent(ctx, tx, contractID, eventTag)
	if err != nil {
		return 0, err
	}
	if err := a.events.SetAccumulated(ctx, tx, contractID, eventTag, sum); err != nil {
		return 0, err
	}
	return sum, nil
}
