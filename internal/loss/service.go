package loss

import (
	"context"
	"strconv"
	"time"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

// Service registers claims and enforces that the loss occurrence falls within
// the contract's effective window and the bound policy's dates.
type Service struct {
	store     *store.Store
	contracts store.ContractStore
	policies  store.PolicyStore
	losses    store.LossStore
	audit     store.AuditStore
}

// New returns a loss Service bound to store s.
func New(s *store.Store) *Service {
	return &Service{
		store:     s,
		contracts: store.NewContractStore(),
		policies:  store.NewPolicyStore(),
		losses:    store.NewLossStore(),
		audit:     store.NewAuditStore(),
	}
}

// Create registers a claim. The loss is linked to the policy's contract; cat_xl
// contracts require a non-empty event_tag so the loss can be accumulated.
func (svc *Service) Create(ctx context.Context, l *domain.Loss) (*domain.Loss, error) {
	if l.ClaimNo == "" {
		return nil, domain.Newf(domain.ErrInvalidArgument, "claim_no is required")
	}
	if l.PaidAmount <= 0 {
		return nil, domain.Newf(domain.ErrInvalidArgument, "paid_amount must be > 0")
	}
	err := svc.store.InTx(ctx, func(tx store.DBTX) error {
		pol, err := svc.policies.Get(ctx, tx, l.PolicyID)
		if err != nil {
			return err
		}
		l.ContractID = pol.ContractID
		c, err := svc.contracts.Get(ctx, tx, pol.ContractID)
		if err != nil {
			return err
		}
		if c.Status != domain.ContractInForce {
			return domain.Newf(domain.ErrInvalidArgument, "contract %s is not in force", c.Code)
		}
		if !validOccurrence(c, pol, l.OccurrenceDate) {
			return domain.Newf(domain.ErrInvalidArgument, "occurrence date must fall within contract and policy windows")
		}
		if c.Type == domain.CatXL && l.EventTag == "" {
			return domain.Newf(domain.ErrInvalidArgument, "cat_xl losses require a non-empty event_tag")
		}
		if err := svc.losses.Create(ctx, tx, l); err != nil {
			return err
		}
		return svc.audit.Append(ctx, tx, "loss.create", "loss", l.ID, lossPayload(l))
	})
	if err != nil {
		return nil, err
	}
	return l, nil
}

// Get returns a loss by id.
func (svc *Service) Get(ctx context.Context, id int64) (*domain.Loss, error) {
	var l *domain.Loss
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		l, err = svc.losses.Get(ctx, tx, id)
		return err
	})
	return l, err
}

// List returns all losses.
func (svc *Service) List(ctx context.Context) ([]*domain.Loss, error) {
	var out []*domain.Loss
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = svc.losses.List(ctx, tx)
		return err
	})
	return out, err
}

func validOccurrence(c *domain.Contract, p *domain.Policy, occ time.Time) bool {
	if occ.IsZero() {
		return false
	}
	if occ.Before(c.StartDate) || occ.After(c.EndDate) {
		return false
	}
	if occ.Before(p.StartDate) || occ.After(p.EndDate) {
		return false
	}
	return true
}

func lossPayload(l *domain.Loss) string {
	return `{"claim_no":"` + l.ClaimNo + `","paid":` + strconv.FormatInt(int64(l.PaidAmount), 10) + `}`
}
