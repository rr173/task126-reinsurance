package alloc

import (
	"context"

	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/store"
)

// ReinstatementPolicy computes reinstatement premiums and decides whether to
// restore the contract limit. The original premium base is the total ceded
// premium across the contract's policies (sum of original_premium * ceded
// rate), which matches real-world reinstatement premium practice and is
// deterministic from stored policy data.
type ReinstatementPolicy struct {
	contracts store.ContractStore
	limits    store.ContractLimitStore
	reins     store.ReinstatementStore
	policies  store.PolicyStore
}

// NewReinstatementPolicy returns a policy bound to store s.
func NewReinstatementPolicy(s *store.Store) *ReinstatementPolicy {
	return &ReinstatementPolicy{
		contracts: store.NewContractStore(),
		limits:    store.NewContractLimitStore(),
		reins:     store.NewReinstatementStore(),
		policies:  store.NewPolicyStore(),
	}
}

// Decision captures whether a reinstatement should fire and the premium to
// charge. A reinstatement fires only when an allocation consumed the entire
// remaining limit AND the contract still has reinstatements available.
type Decision struct {
	Fires           bool
	RestoredAmount  domain.Money // amount of limit restored (== contract limit)
	Premium         domain.Money // reinstatement premium
	RemainingUses   int          // reinstatements available before this decision
	Factor          float64
	OriginalPremium domain.Money // total ceded premium base
}

// Decide evaluates whether the just-consumed limitConsumed triggers a
// reinstatement for contractID within tx. A reinstatement fires only if the
// remaining limit dropped to zero and reinstatements remain.
func (p *ReinstatementPolicy) Decide(ctx context.Context, tx store.DBTX, contractID int64, limitConsumed domain.Money) (*Decision, error) {
	if limitConsumed <= 0 {
		return &Decision{}, nil
	}
	c, err := p.contracts.Get(ctx, tx, contractID)
	if err != nil {
		return nil, err
	}
	d := &Decision{Factor: c.ReinstatementFactor}
	if !c.Type.IsExcess() {
		return d, nil
	}
	cl, err := p.limits.Get(ctx, tx, contractID)
	if err != nil {
		return nil, err
	}
	d.RemainingUses = c.NumReinstatements - cl.ReinstatementsUsed
	d.OriginalPremium, err = p.cededPremiumBase(ctx, tx, c)
	if err != nil {
		return nil, err
	}
	// Fires only when the remaining limit is now zero (allocation drained it).
	if cl.LimitRemaining > 0 {
		return d, nil
	}
	if d.RemainingUses <= 0 {
		return d, nil
	}
	d.Fires = true
	d.RestoredAmount = c.Limit
	// premium = factor * (limitConsumed / limit) * originalPremium, rounded.
	if c.Limit > 0 {
		ratio := float64(limitConsumed) / float64(c.Limit)
		if ratio > 1 {
			ratio = 1
		}
		d.Premium = d.OriginalPremium.MulFrac(c.ReinstatementFactor).MulFrac(ratio)
	}
	return d, nil
}

// cededPremiumBase returns the total ceded premium across the contract's
// policies: proportional uses original_premium * cession_rate per policy;
// excess uses original_premium * ceded_premium_rate per policy.
func (p *ReinstatementPolicy) cededPremiumBase(ctx context.Context, tx store.DBTX, c *domain.Contract) (domain.Money, error) {
	policies, err := p.policies.ListByContract(ctx, tx, c.ID)
	if err != nil {
		return 0, err
	}
	var total domain.Money
	for _, pol := range policies {
		if c.Type.IsProportional() {
			total = total.Add(pol.OriginalPremium.MulFrac(pol.CessionRate))
		} else {
			total = total.Add(pol.OriginalPremium.MulFrac(c.CededPremiumRate))
		}
	}
	return total, nil
}
