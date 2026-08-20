package domain

import (
	"testing"
	"time"
)

// TestMoneyMulFrac checks rounding of proportional money computation.
func TestMoneyMulFrac(t *testing.T) {
	cases := []struct {
		m     Money
		r     float64
		want  Money
	}{
		{10000, 0.5, 5000},
		{10000, 0.333333, 3333}, // round down
		{10000, 0.333334, 3333},
		{1, 0.5, 1},     // 0.5 rounds up (half away from zero)
		{3, 0.5, 2},     // 1.5 -> 2
		{-100, 0.1, -10},
	}
	for _, c := range cases {
		got := c.m.MulFrac(c.r)
		if got != c.want {
			t.Errorf("MulFrac(%d, %v) = %d, want %d", int64(c.m), c.r, int64(got), int64(c.want))
		}
	}
}

func TestMoneyClamp(t *testing.T) {
	if got := Money(50).Clamp(0, 100); got != 50 {
		t.Errorf("Clamp(50,0,100)=%d want 50", int64(got))
	}
	if got := Money(150).Clamp(0, 100); got != 100 {
		t.Errorf("Clamp(150,0,100)=%d want 100", int64(got))
	}
	if got := Money(-5).Clamp(0, 100); got != 0 {
		t.Errorf("Clamp(-5,0,100)=%d want 0", int64(got))
	}
}

func TestRoundCents(t *testing.T) {
	if got := RoundCents(1.4); got != 1 {
		t.Errorf("RoundCents(1.4)=%d want 1", int64(got))
	}
	if got := RoundCents(1.6); got != 2 {
		t.Errorf("RoundCents(1.6)=%d want 2", int64(got))
	}
	if got := RoundCents(-1.6); got != -2 {
		t.Errorf("RoundCents(-1.6)=%d want -2", int64(got))
	}
}

// TestSurplusCessionRate exercises the per-policy cession derivation for
// surplus share contracts: lines ceded = clamp(round((SI-retained)/retained),
// 0, capacity), cession = lines/(1+lines).
func TestSurplusCessionRate(t *testing.T) {
	c := &Contract{
		Type: SurplusShare, RetainedLine: 2000000, TreatyCapacity: 3,
	}
	cases := []struct {
		si   Money
		want float64
	}{
		{8000000, 0.75}, // (8M-2M)/2M=3 lines -> 3/4
		{2000000, 0},    // at retained line -> 0
		{1000000, 0},    // below retained line -> 0
		{3000000, 0.5},  // (3M-2M)/2M=0.5 -> round 1 line -> 1/2
		{50000000, 0.75}, // huge SI -> capped at capacity 3 -> 3/4
	}
	for _, tc := range cases {
		got := c.SurplusCessionRate(tc.si)
		if diff := got - tc.want; diff < -1e-9 || diff > 1e-9 {
			t.Errorf("SurplusCessionRate(%d)=%v want %v", int64(tc.si), got, tc.want)
		}
	}
}

func jan1(year int) time.Time { return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC) }
func jan2(year int) time.Time { return time.Date(year, 1, 2, 0, 0, 0, 0, time.UTC) }
func jan3(year int) time.Time { return time.Date(year, 1, 3, 0, 0, 0, 0, time.UTC) }

func TestContractValidate(t *testing.T) {
	c := &Contract{Type: QuotaShare, CessionRate: 0.5,
		StartDate: jan1(2026), EndDate: jan2(2026)}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid quota_share rejected: %v", err)
	}
	c2 := &Contract{Type: QuotaShare, CessionRate: 1.5,
		StartDate: jan1(2026), EndDate: jan2(2026)}
	if err := c2.Validate(); err == nil {
		t.Fatal("cession_rate > 1 should be rejected")
	}
	c3 := &Contract{Type: PerRiskXL, Limit: 0,
		StartDate: jan1(2026), EndDate: jan2(2026)}
	if err := c3.Validate(); err == nil {
		t.Fatal("excess with zero limit should be rejected")
	}
}

func TestInForceAt(t *testing.T) {
	c := &Contract{
		StartDate: jan1(2026), EndDate: jan2(2026), Status: ContractInForce,
	}
	if !c.InForceAt(jan1(2026)) {
		t.Error("should be in force on start date")
	}
	if c.InForceAt(jan3(2026)) {
		t.Error("should not be in force after end date")
	}
	c.Status = ContractCancelled
	if c.InForceAt(jan1(2026)) {
		t.Error("cancelled contract should not be in force")
	}
}
