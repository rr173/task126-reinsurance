package ceding

import "task126-reinsurance/internal/domain"

// CededPremium returns the premium ceded to the contract for a policy. For
// proportional contracts it is original_premium * cession_rate; for
// non-proportional it is original_premium * ceded_premium_rate.
func CededPremium(c *domain.Contract, p *domain.Policy) domain.Money {
	if c.Type.IsProportional() {
		return p.OriginalPremium.MulFrac(p.CessionRate)
	}
	return p.OriginalPremium.MulFrac(c.CededPremiumRate)
}

// CedingCommission returns the commission owed to the cedant for a ceded
// premium: ceded_premium * ceding_commission_rate.
func CedingCommission(c *domain.Contract, cededPremium domain.Money) domain.Money {
	return cededPremium.MulFrac(c.CedingCommissionRate)
}

// BrokerFee returns the broker commission: ceded_premium * broker_rate.
func BrokerFee(c *domain.Contract, cededPremium domain.Money) domain.Money {
	return cededPremium.MulFrac(c.BrokerRate)
}

// ProportionalRecovery returns the recovery for a loss under a proportional
// contract: paid_amount * cession_rate.
func ProportionalRecovery(p *domain.Policy, loss domain.Money) domain.Money {
	return loss.MulFrac(p.CessionRate)
}
