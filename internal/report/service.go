package report

import (
	"context"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

// Service produces summary reports by aggregating stored data. It is a pure
// read path: no mutations, no transactions needed beyond read consistency.
type Service struct {
	store     *store.Store
	contracts store.ContractStore
	policies  store.PolicyStore
	losses    store.LossStore
	allocs    store.AllocationStore
	reins     store.ReinstatementStore
	events    store.CatastropheEventStore
	bord      store.BordereauxStore
	limits    store.ContractLimitStore
}

// New returns a report Service bound to store s.
func New(s *store.Store) *Service {
	return &Service{
		store:     s,
		contracts: store.NewContractStore(),
		policies:  store.NewPolicyStore(),
		losses:    store.NewLossStore(),
		allocs:    store.NewAllocationStore(),
		reins:     store.NewReinstatementStore(),
		events:    store.NewCatastropheEventStore(),
		bord:      store.NewBordereauxStore(),
		limits:    store.NewContractLimitStore(),
	}
}

// ContractSummary aggregates the running totals for one contract.
type ContractSummary struct {
	Contract           *domain.Contract
	PolicyCount        int
	LossCount          int
	TotalRecovered     domain.Money
	ReinstatementsUsed int
	LimitRemaining     domain.Money
	BordereauxCount    int
	NetBalance         domain.Money // across all settled bordereaux
}

// Contract returns a summary for contractID.
func (svc *Service) Contract(ctx context.Context, contractID int64) (*ContractSummary, error) {
	var s ContractSummary
	err := svc.store.InTx(ctx, func(tx store.DBTX) error {
		c, err := svc.contracts.Get(ctx, tx, contractID)
		if err != nil {
			return err
		}
		s.Contract = c
		pols, err := svc.policies.ListByContract(ctx, tx, contractID)
		if err != nil {
			return err
		}
		s.PolicyCount = len(pols)
		losses, err := svc.losses.ListByContract(ctx, tx, contractID)
		if err != nil {
			return err
		}
		s.LossCount = len(losses)
		s.TotalRecovered, err = svc.allocs.SumRecoveredByContract(ctx, tx, contractID)
		if err != nil {
			return err
		}
		if c.Type.IsExcess() {
			cl, err := svc.limits.Get(ctx, tx, contractID)
			if err != nil {
				return err
			}
			s.LimitRemaining = cl.LimitRemaining
			s.ReinstatementsUsed = cl.ReinstatementsUsed
		}
		bords, err := svc.bord.ListByContract(ctx, tx, contractID)
		if err != nil {
			return err
		}
		s.BordereauxCount = len(bords)
		for _, b := range bords {
			if b.Status == domain.BordereauxSettled {
				s.NetBalance = s.NetBalance.Add(b.NetBalance)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// LossReport summarises a loss and its allocations.
type LossReport struct {
	Loss        *domain.Loss
	Allocations []*domain.Allocation
	NetRetained domain.Money // paid - total recovered
}

// Loss returns a report for lossID.
func (svc *Service) Loss(ctx context.Context, lossID int64) (*LossReport, error) {
	var r LossReport
	err := svc.store.InTx(ctx, func(tx store.DBTX) error {
		l, err := svc.losses.Get(ctx, tx, lossID)
		if err != nil {
			return err
		}
		r.Loss = l
		allocs, err := svc.allocs.ListByLoss(ctx, tx, lossID)
		if err != nil {
			return err
		}
		r.Allocations = allocs
		rec, err := svc.allocs.SumRecoveredByLoss(ctx, tx, l.ID)
		if err != nil {
			return err
		}
		r.NetRetained = l.PaidAmount.Sub(rec)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// BordereauxRow is one row of the bordereaux report.
type BordereauxRow struct {
	*Bordereaux
}

// Bordereaux is an alias re-export so callers don't import store for the row.
type Bordereaux = domain.Bordereaux

// BordereauxAll returns every bordereaux.
func (svc *Service) BordereauxAll(ctx context.Context) ([]*Bordereaux, error) {
	var out []*Bordereaux
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = svc.bord.List(ctx, tx)
		return err
	})
	return out, err
}

// LossesAll returns every loss with its allocations.
func (svc *Service) LossesAll(ctx context.Context) ([]*LossReport, error) {
	var out []*LossReport
	err := svc.store.InTx(ctx, func(tx store.DBTX) error {
		losses, err := svc.losses.List(ctx, tx)
		if err != nil {
			return err
		}
		for _, l := range losses {
			allocs, err := svc.allocs.ListByLoss(ctx, tx, l.ID)
			if err != nil {
				return err
			}
			rec, err := svc.allocs.SumRecoveredByLoss(ctx, tx, l.ID)
			if err != nil {
				return err
			}
			out = append(out, &LossReport{Loss: l, Allocations: allocs, NetRetained: l.PaidAmount.Sub(rec)})
		}
		return nil
	})
	return out, err
}

// EventsForContract returns cat events for a contract.
func (svc *Service) EventsForContract(ctx context.Context, contractID int64) ([]*domain.CatastropheEvent, error) {
	var out []*domain.CatastropheEvent
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = svc.events.ListByContract(ctx, tx, contractID)
		return err
	})
	return out, err
}
