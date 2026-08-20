package contract

import (
	"context"
	"time"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

// Service manages treaty lifecycle: registration and status transitions. It is
// the single entry point for contract mutations; allocation and billing reach
// the same store but via their own services.
type Service struct {
	store  *store.Store
	contracts store.ContractStore
	audit  store.AuditStore
}

// New returns a contract Service bound to store s.
func New(s *store.Store) *Service {
	return &Service{
		store:     s,
		contracts: store.NewContractStore(),
		audit:     store.NewAuditStore(),
	}
}

// Create registers a new treaty after type-specific validation.
func (svc *Service) Create(ctx context.Context, c *domain.Contract) (*domain.Contract, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if c.Currency == "" {
		c.Currency = "CNY"
	}
	err := svc.store.InTx(ctx, func(tx store.DBTX) error {
		if err := svc.contracts.Create(ctx, tx, c); err != nil {
			return err
		}
		return svc.audit.Append(ctx, tx, "contract.create", "contract", c.ID, contractPayload(c))
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Get returns a contract by id.
func (svc *Service) Get(ctx context.Context, id int64) (*domain.Contract, error) {
	var c *domain.Contract
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		c, err = svc.contracts.Get(ctx, tx, id)
		return err
	})
	return c, err
}

// List returns all contracts.
func (svc *Service) List(ctx context.Context) ([]*domain.Contract, error) {
	var out []*domain.Contract
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		out, err = svc.contracts.List(ctx, tx)
		return err
	})
	return out, err
}

// TransitionStatus moves a contract from its current status to next. Allowed
// transitions: in_force -> expired|cancelled. expired/cancelled are terminal.
func (svc *Service) TransitionStatus(ctx context.Context, id int64, next domain.ContractStatus) (*domain.Contract, error) {
	if !next.Valid() {
		return nil, domain.Newf(domain.ErrInvalidArgument, "invalid contract status %q", next)
	}
	var c *domain.Contract
	err := svc.store.InTx(ctx, func(tx store.DBTX) (err error) {
		c, err = svc.contracts.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		if !validTransition(c.Status, next) {
			return domain.Newf(domain.ErrInvalidArgument, "contract %s cannot transition to %s", c.Status, next)
		}
		if err := svc.contracts.UpdateStatus(ctx, tx, id, next); err != nil {
			return err
		}
		c.Status = next
		return svc.audit.Append(ctx, tx, "contract.transition", "contract", id, statusPayload(next))
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func validTransition(from, to domain.ContractStatus) bool {
	switch from {
	case domain.ContractInForce:
		return to == domain.ContractExpired || to == domain.ContractCancelled
	}
	return false
}

// EffectiveAt reports whether the contract covers occurrence t. Contracts whose
// status is expired or cancelled are not effective even if t falls inside the
// date window.
func EffectiveAt(c *domain.Contract, t time.Time) bool {
	return c.InForceAt(t)
}

func contractPayload(c *domain.Contract) string {
	return `{"code":"` + c.Code + `","type":"` + string(c.Type) + `","status":"` + string(c.Status) + `"}`
}

func statusPayload(s domain.ContractStatus) string {
	return `{"status":"` + string(s) + `"}`
}
