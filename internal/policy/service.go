package policy

import (
	"context"
	"strconv"
	"time"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

// Service registers direct policies and derives their cession rate from the
// bound contract. It enforces that the contract is in force and that the policy
// dates fall within the contract's effective window.
type Service struct {
	store     *store.Store
	contracts store.ContractStore
	policies  store.PolicyStore
	audit     store.AuditStore
}

// New returns a policy Service bound to store s.
func New(s *store.Store) *Service {
	return &Service{
		store:     s,
		contracts: store.NewContractStore(),
		policies:  store.NewPolicyStore(),
		audit:     store.NewAuditStore(),
	}
}

// Create registers a policy. For surplus_share contracts the cession rate is
// derived from the sum insured; for quota_share it mirrors the contract rate;
// for excess types the cession rate is unused (set to 0).
func (svc *Service) Create(ctx context.Context, p *domain.Policy) (*domain.Policy, error) {
	if p.PolicyNo == "" {
		return nil, domain.Newf(domain.ErrInvalidArgument, "policy_no is required")
	}
	if p.SumInsured <= 0 {
		return nil, domain.Newf(domain.ErrInvalidArgument, "sum_insured must be > 0")
	}
	if p.OriginalPremium < 0 {
		return nil, domain.Newf(domain.ErrInvalidArgument, "original_premium must be >= 0")
	}
	err := svc.store.InTx(ctx, func(tx store.DBTX) error {
		c, err := svc.contracts.Get(ctx, tx, p.ContractID)
		if err != nil {
			return err
		}
		if c.Status != domain.ContractInForce {
			return domain.Newf(domain.ErrInvalidArgument, "contract %s is not in force", c.Code)
		}
		if !withinContractWindow(c, p.StartDate, p.EndDate) {
			return domain.Newf(domain.ErrInvalidArgument, "policy dates must fall within contract window")
		}
		switch c.Type {
		case domain.QuotaShare:
			p.CessionRate = c.CessionRate
		case domain.SurplusShare:
			p.CessionRate = c.SurplusCessionRate(p.SumInsured)
		default:
			p.CessionRate = 0
		}
		if err := svc.policies.Create(ctx, tx, p); err != nil {
			return err
		}
		return svc.audit.Append(ctx, tx, "policy.create", "policy", p.ID, policyPayload(p))
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Get returns a policy by id.
func (svc *Service) Get(ctx context.Context, id int64) (*domain.Policy, error) {
	var p *domain.Policy
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		p, err = svc.policies.Get(ctx, tx, id)
		return err
	})
	return p, err
}

// List returns all policies.
func (svc *Service) List(ctx context.Context) ([]*domain.Policy, error) {
	var out []*domain.Policy
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = svc.policies.List(ctx, tx)
		return err
	})
	return out, err
}

// ListByContract returns policies for a contract.
func (svc *Service) ListByContract(ctx context.Context, contractID int64) ([]*domain.Policy, error) {
	var out []*domain.Policy
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = svc.policies.ListByContract(ctx, tx, contractID)
		return err
	})
	return out, err
}

func withinContractWindow(c *domain.Contract, start, end time.Time) bool {
	if start.IsZero() || end.IsZero() {
		return false
	}
	if !end.After(start) {
		return false
	}
	// Policy window must be within [contract.start, contract.end].
	if start.Before(c.StartDate) {
		return false
	}
	if end.After(c.EndDate) {
		return false
	}
	return true
}

func policyPayload(p *domain.Policy) string {
	return `{"policy_no":"` + p.PolicyNo + `","contract_id":` + strconv.FormatInt(p.ContractID, 10) + `}`
}
