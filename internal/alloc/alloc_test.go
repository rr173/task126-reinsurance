package alloc

import (
	"testing"

	"task126-reinsurance/internal/domain"
)

// TestExcessLayer drills the excess-layer math: attachment absorbs the
// retained portion, the layer pays up to the remaining limit, and anything
// above the limit is retained by the cedant.
func TestExcessLayer(t *testing.T) {
	cases := []struct {
		name          string
		loss          domain.Money
		attachment    domain.Money
		remaining     domain.Money
		wantAtt       domain.Money
		wantRecovery  domain.Money
		wantAbove     domain.Money
	}{
		{"below attachment", 3000, 5000, 10000, 3000, 0, 0},
		{"in layer", 8000, 5000, 10000, 5000, 3000, 0},
		{"capped at limit", 20000, 5000, 10000, 5000, 10000, 5000},
		{"partial remaining", 20000, 5000, 3000, 5000, 3000, 12000},
		{"zero remaining", 5000, 0, 0, 0, 0, 5000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			att, rec, above := ExcessLayer(tc.loss, tc.attachment, tc.remaining)
			if att != tc.wantAtt {
				t.Errorf("attachment: got %d want %d", int64(att), int64(tc.wantAtt))
			}
			if rec != tc.wantRecovery {
				t.Errorf("recovery: got %d want %d", int64(rec), int64(tc.wantRecovery))
			}
			if above != tc.wantAbove {
				t.Errorf("retentionAbove: got %d want %d", int64(above), int64(tc.wantAbove))
			}
		})
	}
}

func TestProportionalRecovery(t *testing.T) {
	c := &domain.Contract{Type: domain.QuotaShare, CedingCommissionRate: 0.1, BrokerRate: 0}
	p := &domain.Policy{OriginalPremium: 400000, CessionRate: 0.5}
	res := Proportional(c, p, 1000000)
	if res.Recovery != 500000 {
		t.Errorf("recovery: got %d want 500000", int64(res.Recovery))
	}
	if res.CededPremium != 200000 {
		t.Errorf("ceded premium: got %d want 200000", int64(res.CededPremium))
	}
	if res.CedingCommission != 20000 {
		t.Errorf("commission: got %d want 20000", int64(res.CedingCommission))
	}
}
