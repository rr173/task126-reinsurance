package alloc

import (
	"context"
	"fmt"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

// Engine orchestrates loss allocation across contract types. It is the single
// entry point for recovering a loss (or a catastrophe event) against its
// contract: proportional contracts pay a fixed slice, excess contracts pay the
// layered amount and may trigger a reinstatement. All mutations happen inside
// the caller's transaction.
type Engine struct {
	store     *store.Store
	contracts store.ContractStore
	policies  store.PolicyStore
	losses    store.LossStore
	allocs    store.AllocationStore
	limits    store.ContractLimitStore
	reins     store.ReinstatementStore
	events    store.CatastropheEventStore
	audit     store.AuditStore
	reinsPol  *ReinstatementPolicy
	catAcc    *CatastropheAccumulator
}

// New returns an Engine bound to store s.
func New(s *store.Store) *Engine {
	return &Engine{
		store:     s,
		contracts: store.NewContractStore(),
		policies:  store.NewPolicyStore(),
		losses:    store.NewLossStore(),
		allocs:    store.NewAllocationStore(),
		limits:    store.NewContractLimitStore(),
		reins:     store.NewReinstatementStore(),
		events:    store.NewCatastropheEventStore(),
		audit:     store.NewAuditStore(),
		reinsPol:  NewReinstatementPolicy(s),
		catAcc:    NewCatastropheAccumulator(s),
	}
}

// AllocateLoss recovers a loss against its contract and records the allocation.
// It advances the loss status from open -> allocated. If the loss is already
// allocated/closed it returns the existing allocations without re-running (idem-
// potence: a loss is never recovered twice against the same contract).
func (e *Engine) AllocateLoss(ctx context.Context, lossID int64) ([]*domain.Allocation, error) {
	var out []*domain.Allocation
	err := e.store.InTx(ctx, func(tx store.DBTX) error {
		loss, err := e.losses.Get(ctx, tx, lossID)
		if err != nil {
			return err
		}
		// Idempotency: already-allocated losses return their existing layers.
		if loss.Status == domain.LossAllocated || loss.Status == domain.LossClosed {
			out, err = e.allocs.ListByLoss(ctx, tx, loss.ID)
			return err
		}
		c, err := e.contracts.Get(ctx, tx, loss.ContractID)
		if err != nil {
			return err
		}
		pol, err := e.policies.Get(ctx, tx, loss.PolicyID)
		if err != nil {
			return err
		}
		// Contract must be in force at the occurrence date.
		if !c.InForceAt(loss.OccurrenceDate) {
			return domain.Newf(domain.ErrInvalidArgument,
				"contract %s not in force at occurrence %s", c.Code, loss.OccurrenceDate.Format("2006-01-02"))
		}
		var allocs []*domain.Allocation
		var reinDecision *Decision
		switch c.Type {
		case domain.QuotaShare, domain.SurplusShare:
			allocs = e.allocateProportional(ctx, tx, c, pol, loss)
		case domain.PerRiskXL:
			allocs, reinDecision, err = e.allocateExcess(ctx, tx, c, loss, loss.PaidAmount)
			if err != nil {
				return err
			}
		case domain.CatXL:
			// CatXL is not allocated per-loss; it is allocated per-event. A per-
			// loss allocation against a cat contract is a no-op that returns the
			// existing layers (which are written by AllocateEvent).
			allocs, err = e.allocs.ListByLoss(ctx, tx, loss.ID)
			if err != nil {
				return err
			}
		default:
			return domain.Newf(domain.ErrInvalidArgument, "unknown contract type %q", c.Type)
		}
		// Apply reinstatement if an excess layer drained the limit.
		if reinDecision != nil && reinDecision.Fires {
			if err := e.applyReinstatement(ctx, tx, c, loss, reinDecision); err != nil {
				return err
			}
		}
		// Persist allocations and advance the loss.
		for _, a := range allocs {
			if err := e.allocs.Create(ctx, tx, a); err != nil {
				return err
			}
		}
		if err := e.losses.UpdateStatus(ctx, tx, loss.ID, domain.LossAllocated); err != nil {
			return err
		}
		if err := e.audit.Append(ctx, tx, "loss.allocate", "loss", loss.ID,
			fmt.Sprintf(`{"loss_id":%d,"allocations":%d}`, loss.ID, len(allocs))); err != nil {
			return err
		}
		out = allocs
		return nil
	})
	return out, err
}

// allocateProportional builds a single proportional allocation layer.
func (e *Engine) allocateProportional(ctx context.Context, tx store.DBTX, c *domain.Contract, p *domain.Policy, l *domain.Loss) []*domain.Allocation {
	res := Proportional(c, p, l.PaidAmount)
	return []*domain.Allocation{{
		LossID:          l.ID,
		ContractID:      c.ID,
		ContractType:    c.Type,
		Reinsurer:       c.Code,
		RecoveredAmount: res.Recovery,
		Kind:            domain.AllocationProportional,
	}}
}

// allocateExcess builds one excess layer for a per_risk_xl contract, decrements
// the remaining limit and returns the reinstatement decision (caller applies it
// after persisting the allocation).
func (e *Engine) allocateExcess(ctx context.Context, tx store.DBTX, c *domain.Contract, l *domain.Loss, base domain.Money) ([]*domain.Allocation, *Decision, error) {
	cl, err := e.limits.Get(ctx, tx, c.ID)
	if err != nil {
		return nil, nil, err
	}
	attachmentConsumed, layerRecovery, retentionAbove := ExcessLayer(base, c.AttachmentPoint, cl.LimitRemaining)
	if layerRecovery <= 0 {
		// Nothing recoverable; the cedant retains the entire loss.
		return nil, nil, nil
	}
	if err := e.limits.DecrementLimit(ctx, tx, c.ID, layerRecovery); err != nil {
		return nil, nil, err
	}
	alloc := &domain.Allocation{
		LossID:             l.ID,
		ContractID:         c.ID,
		ContractType:       c.Type,
		Reinsurer:          c.Code,
		RecoveredAmount:    layerRecovery,
		AttachmentConsumed: attachmentConsumed,
		LimitConsumed:      layerRecovery,
		Kind:               domain.AllocationExcessLayer,
	}
	_ = retentionAbove
	// Reinstatement decision is taken AFTER the decrement so remaining==0 is seen.
	decision, err := e.reinsPol.Decide(ctx, tx, c.ID, layerRecovery)
	if err != nil {
		return nil, nil, err
	}
	return []*domain.Allocation{alloc}, decision, nil
}

// applyReinstatement restores the limit and records the reinstatement.
func (e *Engine) applyReinstatement(ctx context.Context, tx store.DBTX, c *domain.Contract, l *domain.Loss, d *Decision) error {
	if err := e.limits.RestoreLimit(ctx, tx, c.ID, c.Limit); err != nil {
		return err
	}
	seq, err := e.reins.CountByContract(ctx, tx, c.ID)
	if err != nil {
		return err
	}
	r := &domain.Reinstatement{
		ContractID:         c.ID,
		LossID:            l.ID,
		Seq:               seq + 1,
		RestoredAmount:    d.RestoredAmount,
		Premium:           d.Premium,
		ReinstatementFactor: d.Factor,
		OriginalPremium:   d.OriginalPremium,
	}
	if err := e.reins.Create(ctx, tx, r); err != nil {
		return err
	}
	return e.audit.Append(ctx, tx, "reinstatement.create", "reinstatement", r.ID,
		fmt.Sprintf(`{"contract_id":%d,"seq":%d,"premium":%d}`, c.ID, r.Seq, int64(r.Premium)))
}

// AllocateEvent recovers an accumulated catastrophe event against a cat_xl
// contract. It accumulates all paid losses sharing the event tag under the
// contract, applies the excess layer once, marks the event allocated (no double
// recovery) and triggers a reinstatement if the limit drained.
func (e *Engine) AllocateEvent(ctx context.Context, contractID int64, eventTag string) ([]*domain.Allocation, error) {
	var out []*domain.Allocation
	err := e.store.InTx(ctx, func(tx store.DBTX) error {
		c, err := e.contracts.Get(ctx, tx, contractID)
		if err != nil {
			return err
		}
		if c.Type != domain.CatXL {
			return domain.Newf(domain.ErrInvalidArgument, "contract %s is not cat_xl", c.Code)
		}
		if eventTag == "" {
			return domain.Newf(domain.ErrInvalidArgument, "event_tag required")
		}
		// Prevent double recovery of an already-allocated event.
		if err := e.catAcc.MustNotBeAllocated(ctx, tx, contractID, eventTag); err != nil {
			return err
		}
		// The accumulated loss is the authoritative sum of paid losses.
		acc, err := e.catAcc.Reconcile(ctx, tx, contractID, eventTag)
		if err != nil {
			return err
		}
		if acc <= 0 {
			return domain.Newf(domain.ErrInvalidArgument, "no accumulated loss for event %q", eventTag)
		}
		// Use the first loss as the anchor for the allocation record (cat layers
		// are attributed to the event, recorded against the anchoring loss).
		losses, err := e.losses.ListByContractEvent(ctx, tx, contractID, eventTag)
		if err != nil {
			return err
		}
		if len(losses) == 0 {
			return domain.Newf(domain.ErrInvalidArgument, "no losses for event %q", eventTag)
		}
		// Contracts must be in force across the event window (checked per loss).
		for _, l := range losses {
			if !c.InForceAt(l.OccurrenceDate) {
				return domain.Newf(domain.ErrInvalidArgument,
					"contract %s not in force at occurrence %s for loss %s", c.Code, l.OccurrenceDate.Format("2006-01-02"), l.ClaimNo)
			}
		}
		anchor := losses[0]
		// Excess layer on the accumulated amount.
		cl, err := e.limits.Get(ctx, tx, c.ID)
		if err != nil {
			return err
		}
		attachmentConsumed, layerRecovery, _ := ExcessLayer(acc, c.AttachmentPoint, cl.LimitRemaining)
		var eventRecovered domain.Money
		if layerRecovery > 0 {
			if err := e.limits.DecrementLimit(ctx, tx, c.ID, layerRecovery); err != nil {
				return err
			}
			alloc := &domain.Allocation{
				LossID:             anchor.ID,
				ContractID:         c.ID,
				ContractType:       c.Type,
				Reinsurer:          c.Code,
				RecoveredAmount:    layerRecovery,
				AttachmentConsumed: attachmentConsumed,
				LimitConsumed:      layerRecovery,
				Kind:               domain.AllocationCatLayer,
			}
			if err := e.allocs.Create(ctx, tx, alloc); err != nil {
				return err
			}
			out = append(out, alloc)
			eventRecovered = layerRecovery
			// Trigger reinstatement if drained.
			decision, err := e.reinsPol.Decide(ctx, tx, c.ID, layerRecovery)
			if err != nil {
				return err
			}
			if decision.Fires {
				if err := e.applyReinstatement(ctx, tx, c, anchor, decision); err != nil {
					return err
				}
			}
		}
		// Mark all event losses allocated so they are not re-opened per-loss.
		for _, l := range losses {
			if err := e.losses.UpdateStatus(ctx, tx, l.ID, domain.LossAllocated); err != nil {
				return err
			}
		}
		if err := e.catAcc.MarkAllocated(ctx, tx, contractID, eventTag); err != nil {
			return err
		}
		return e.audit.Append(ctx, tx, "event.allocate", "contract", contractID,
			fmt.Sprintf(`{"event_tag":"%s","recovered":%d}`, eventTag, int64(eventRecovered)))
	})
	return out, err
}

// AllocationsForLoss returns existing allocations for a loss.
func (e *Engine) AllocationsForLoss(ctx context.Context, lossID int64) ([]*domain.Allocation, error) {
	var out []*domain.Allocation
	err := e.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = e.allocs.ListByLoss(ctx, tx, lossID)
		return err
	})
	return out, err
}

// ContractEvents lists catastrophe events for a contract.
func (e *Engine) ContractEvents(ctx context.Context, contractID int64) ([]*domain.CatastropheEvent, error) {
	var out []*domain.CatastropheEvent
	err := e.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = e.events.ListByContract(ctx, tx, contractID)
		return err
	})
	return out, err
}

// LimitsFor returns the running limit state of an excess contract.
func (e *Engine) LimitsFor(ctx context.Context, contractID int64) (*domain.ContractLimit, error) {
	var out *domain.ContractLimit
	err := e.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = e.limits.Get(ctx, tx, contractID)
		return err
	})
	return out, err
}
