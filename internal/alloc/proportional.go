package alloc

import (
	"task126-reinsurance/internal/ceding"
	"task126-reinsurance/internal/domain"
)

// ProportionalResult is the outcome of allocating a loss against a proportional
// (quota_share / surplus_share) contract.
type ProportionalResult struct {
	Recovery domain.Money
	// CededPremium and Commission are the policy's contributions for the
	// bordereaux (computed once but surfaced here for the allocation ledger).
	CededPremium    domain.Money
	CedingCommission domain.Money
	BrokerFee       domain.Money
}

// Proportional allocates a loss against a proportional contract. The recovery
// is paid_amount * policy cession_rate. Ceded premium and commissions are
// computed for the policy so the bordereaux can aggregate them.
func Proportional(c *domain.Contract, p *domain.Policy, loss domain.Money) ProportionalResult {
	recovery := ceding.ProportionalRecovery(p, loss)
	ceded := cedingForContract(c, p)
	return ProportionalResult{
		Recovery:         recovery,
		CededPremium:     ceded,
		CedingCommission: ceded.MulFrac(c.CedingCommissionRate),
		BrokerFee:        ceded.MulFrac(c.BrokerRate),
	}
}

func cedingForContract(c *domain.Contract, p *domain.Policy) domain.Money {
	return ceding.CededPremium(c, p)
}
