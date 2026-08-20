package bordereaux

import (
	"context"
	"fmt"
	"time"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

// Service computes and settles quarterly accounts. Settlement aggregates the
// ceded premium, recovered losses, commissions and reinstatement premiums for
// a contract in a quarter, writes the totals, marks the account settled and
// locks it. Reopening is allowed only when no later quarter is settled.
type Service struct {
	store      *store.Store
	contracts  store.ContractStore
	policies   store.PolicyStore
	losses     store.LossStore
	allocs     store.AllocationStore
	reins      store.ReinstatementStore
	bord       store.BordereauxStore
	audit      store.AuditStore
}

// New returns a bordereaux Service bound to store s.
func New(s *store.Store) *Service {
	return &Service{
		store:     s,
		contracts: store.NewContractStore(),
		policies:  store.NewPolicyStore(),
		losses:    store.NewLossStore(),
		allocs:    store.NewAllocationStore(),
		reins:     store.NewReinstatementStore(),
		bord:      store.NewBordereauxStore(),
		audit:     store.NewAuditStore(),
	}
}

// Aggregates holds the computed totals for a (contract, quarter) account.
type Aggregates struct {
	CededPremium         domain.Money
	RecoveredLoss        domain.Money
	CedingCommission     domain.Money
	BrokerFee            domain.Money
	ReinstatementPremium domain.Money
	NetBalance           domain.Money
}

// compute aggregates a contract's quarter from source data.
func (svc *Service) compute(ctx context.Context, tx store.DBTX, contractID int64, year, quarter int) (Aggregates, error) {
	var agg Aggregates
	c, err := svc.contracts.Get(ctx, tx, contractID)
	if err != nil {
		return agg, err
	}
	policies, err := svc.policies.ListByContract(ctx, tx, contractID)
	if err != nil {
		return agg, err
	}
	for _, p := range policies {
		// Ceded premium is attributed to the quarter in which the policy is
		// written/attached (its start date). This is deterministic and avoids
		// double-counting a year-long policy across quarters.
		if !inQuarter(p.StartDate, year, quarter) {
			continue
		}
		ceded := cededPremium(c, p)
		agg.CededPremium = agg.CededPremium.Add(ceded)
		agg.CedingCommission = agg.CedingCommission.Add(ceded.MulFrac(c.CedingCommissionRate))
		agg.BrokerFee = agg.BrokerFee.Add(ceded.MulFrac(c.BrokerRate))
	}
	// Recovered losses attributed to the quarter by occurrence date.
	recovered, err := svc.allocs.SumRecoveredByContractInQuarter(ctx, tx, contractID, year, quarter)
	if err != nil {
		return agg, err
	}
	agg.RecoveredLoss = recovered
	// Reinstatement premiums triggered in the quarter.
	reinPrem, err := svc.reins.SumPremiumByContractInQuarter(ctx, tx, contractID, year, quarter)
	if err != nil {
		return agg, err
	}
	agg.ReinstatementPremium = reinPrem
	// Net = ceded premium + reinstatement premium - recovered - commission - broker fee.
	// A positive net means the cedant owes the reinsurer; negative means the
	// reinsurer owes the cedant (recoveries exceed premiums).
	agg.NetBalance = agg.CededPremium.
		Add(agg.ReinstatementPremium).
		Sub(agg.RecoveredLoss).
		Sub(agg.CedingCommission).
		Sub(agg.BrokerFee)
	return agg, nil
}

// cededPremium returns the premium ceded for a policy in the quarter window.
func cededPremium(c *domain.Contract, p *domain.Policy) domain.Money {
	if c.Type.IsProportional() {
		return p.OriginalPremium.MulFrac(p.CessionRate)
	}
	return p.OriginalPremium.MulFrac(c.CededPremiumRate)
}

// inQuarter reports whether t falls in the calendar quarter.
func inQuarter(t time.Time, year, quarter int) bool {
	_, q := domain.QuarterOf(t)
	return t.UTC().Year() == year && q == quarter
}

// Ensure returns the open bordereaux for a (contract, quarter), creating it if
// absent. If the quarter is already settled, the settled record is returned
// unchanged.
func (svc *Service) Ensure(ctx context.Context, contractID int64, year, quarter int) (*domain.Bordereaux, error) {
	if quarter < 1 || quarter > 4 {
		return nil, domain.Newf(domain.ErrInvalidArgument, "quarter must be 1..4")
	}
	var b *domain.Bordereaux
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		b, err = svc.bord.Upsert(ctx, tx, contractID, year, quarter)
		return err
	})
	return b, err
}

// Settle aggregates the quarter and locks the account. Settled accounts cannot
// be settled again; call Reopen first.
func (svc *Service) Settle(ctx context.Context, id int64) (*domain.Bordereaux, error) {
	var b *domain.Bordereaux
	err := svc.store.InTx(ctx, func(tx store.DBTX) error {
		cur, err := svc.bord.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur.Status == domain.BordereauxSettled {
			return domain.Newf(domain.ErrConflict, "bordereaux %d already settled", id)
		}
		agg, err := svc.compute(ctx, tx, cur.ContractID, cur.PeriodYear, cur.PeriodQuarter)
		if err != nil {
			return err
		}
		if err := svc.bord.Settle(ctx, tx, id, store.BordereauxAggregates(agg)); err != nil {
			return err
		}
		b, err = svc.bord.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		return svc.audit.Append(ctx, tx, "bordereaux.settle", "bordereaux", id,
			fmt.Sprintf(`{"net_balance":%d}`, int64(b.NetBalance)))
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Reopen unlocks a settled account, allowed only when no later quarter is
// settled. Reopening returns the account to open and zeroes its aggregates so
// they are recomputed on the next settlement.
func (svc *Service) Reopen(ctx context.Context, id int64) (*domain.Bordereaux, error) {
	var b *domain.Bordereaux
	err := svc.store.InTx(ctx, func(tx store.DBTX) error {
		cur, err := svc.bord.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur.Status != domain.BordereauxSettled {
			return domain.Newf(domain.ErrConflict, "bordereaux %d is not settled", id)
		}
		later, err := svc.bord.HasLaterSettled(ctx, tx, cur.ContractID, cur.PeriodYear, cur.PeriodQuarter)
		if err != nil {
			return err
		}
		if later {
			return domain.Newf(domain.ErrConflict, "a later quarter is already settled; cannot reopen %d", id)
		}
		if err := svc.bord.Reopen(ctx, tx, id); err != nil {
			return err
		}
		b, err = svc.bord.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		return svc.audit.Append(ctx, tx, "bordereaux.reopen", "bordereaux", id, "{}")
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Get returns a bordereaux by id.
func (svc *Service) Get(ctx context.Context, id int64) (*domain.Bordereaux, error) {
	var b *domain.Bordereaux
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		b, err = svc.bord.Get(ctx, tx, id)
		return err
	})
	return b, err
}

// List returns all bordereaux.
func (svc *Service) List(ctx context.Context) ([]*domain.Bordereaux, error) {
	var out []*domain.Bordereaux
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = svc.bord.List(ctx, tx)
		return err
	})
	return out, err
}

// ListByContract returns bordereaux for a contract.
func (svc *Service) ListByContract(ctx context.Context, contractID int64) ([]*domain.Bordereaux, error) {
	var out []*domain.Bordereaux
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = svc.bord.ListByContract(ctx, tx, contractID)
		return err
	})
	return out, err
}
