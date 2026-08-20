package ceding

import (
	"testing"

	"task126-reinsurance/internal/domain"
)

func TestCededPremiumProportional(t *testing.T) {
	c := &domain.Contract{Type: domain.QuotaShare}
	p := &domain.Policy{OriginalPremium: 400000, CessionRate: 0.5}
	if got := CededPremium(c, p); got != 200000 {
		t.Errorf("proportional ceded premium: got %d want 200000", int64(got))
	}
}

func TestCededPremiumExcess(t *testing.T) {
	c := &domain.Contract{Type: domain.PerRiskXL, CededPremiumRate: 0.25}
	p := &domain.Policy{OriginalPremium: 400000}
	if got := CededPremium(c, p); got != 100000 {
		t.Errorf("excess ceded premium: got %d want 100000", int64(got))
	}
}

func TestCedingCommission(t *testing.T) {
	c := &domain.Contract{CedingCommissionRate: 0.1}
	if got := CedingCommission(c, 200000); got != 20000 {
		t.Errorf("ceding commission: got %d want 20000", int64(got))
	}
}

func TestBrokerFee(t *testing.T) {
	c := &domain.Contract{BrokerRate: 0.02}
	if got := BrokerFee(c, 200000); got != 4000 {
		t.Errorf("broker fee: got %d want 4000", int64(got))
	}
}

func TestProportionalRecoveryMethod(t *testing.T) {
	p := &domain.Policy{CessionRate: 0.5}
	if got := ProportionalRecovery(p, 1000000); got != 500000 {
		t.Errorf("recovery: got %d want 500000", int64(got))
	}
}
