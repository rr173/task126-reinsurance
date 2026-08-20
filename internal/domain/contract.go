package domain

import (
	"strings"
	"time"
)

// ContractType enumerates the four supported reinsurance contract kinds.
type ContractType string

const (
	// QuotaShare: proportional. Every policy cedes a fixed percentage
	// (cession_rate) of premium and loss.
	QuotaShare ContractType = "quota_share"
	// SurplusShare: proportional. Cession rate is derived per policy from
	// (sum_insured, retained_line, treaty_capacity).
	SurplusShare ContractType = "surplus_share"
	// PerRiskXL: non-proportional excess-of-loss on a single risk/loss. Pays
	// the portion of each loss above attachment_point, capped at limit.
	PerRiskXL ContractType = "per_risk_xl"
	// CatXL: non-proportional catastrophe excess-of-loss. Losses sharing an
	// event_tag accumulate before attachment_point/limit apply.
	CatXL ContractType = "cat_xl"
)

// IsProportional reports whether the contract type cedes a fixed/calculated
// percentage of every premium and loss (vs. layered excess).
func (t ContractType) IsProportional() bool {
	return t == QuotaShare || t == SurplusShare
}

// IsExcess reports whether the contract type uses attachment/limit layering.
func (t ContractType) IsExcess() bool {
	return t == PerRiskXL || t == CatXL
}

// CanonicalEventTag makes equivalent catastrophe labels resolve to one event.
func CanonicalEventTag(tag string) string { return strings.ToLower(strings.TrimSpace(tag)) }

// ContractStatus is the lifecycle state of a treaty.
type ContractStatus string

const (
	ContractInForce   ContractStatus = "in_force"
	ContractExpired   ContractStatus = "expired"
	ContractCancelled ContractStatus = "cancelled"
)

// Valid reports whether s is a known contract status.
func (s ContractStatus) Valid() bool {
	switch s {
	case ContractInForce, ContractExpired, ContractCancelled:
		return true
	}
	return false
}

// Contract is a reinsurance treaty.
type Contract struct {
	ID                   int64
	Code                 string
	Type                 ContractType
	AttachmentPoint      Money   // excess types only
	Limit                Money   // excess types only
	CessionRate          float64 // quota_share
	RetainedLine         Money   // surplus_share self-retained line
	TreatyCapacity       int     // surplus_share max ceded lines
	NumReinstatements    int     // excess types; 0 = no reinstatement
	ReinstatementFactor  float64 // excess types
	CedingCommissionRate float64 // proportional (and optional excess)
	BrokerRate           float64
	CededPremiumRate     float64 // non-proportional: ceded_premium = original_premium * rate
	Currency             string
	StartDate            time.Time
	EndDate              time.Time
	Status               ContractStatus
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Validate checks type-specific field consistency.
func (c *Contract) Validate() error {
	switch c.Type {
	case QuotaShare:
		if c.CessionRate <= 0 || c.CessionRate > 1 {
			return Newf(ErrInvalidArgument, "quota_share cession_rate must be in (0,1], got %v", c.CessionRate)
		}
	case SurplusShare:
		if c.RetainedLine <= 0 {
			return Newf(ErrInvalidArgument, "surplus_share retained_line must be > 0")
		}
		if c.TreatyCapacity <= 0 {
			return Newf(ErrInvalidArgument, "surplus_share treaty_capacity must be > 0")
		}
	case PerRiskXL, CatXL:
		if c.Limit <= 0 {
			return Newf(ErrInvalidArgument, "excess contract limit must be > 0")
		}
		if c.AttachmentPoint < 0 {
			return Newf(ErrInvalidArgument, "excess attachment_point must be >= 0")
		}
		if c.NumReinstatements < 0 {
			return Newf(ErrInvalidArgument, "num_reinstatements must be >= 0")
		}
		if c.ReinstatementFactor < 0 {
			return Newf(ErrInvalidArgument, "reinstatement_factor must be >= 0")
		}
	default:
		return Newf(ErrInvalidArgument, "unknown contract type %q", c.Type)
	}
	if c.CedingCommissionRate < 0 || c.CedingCommissionRate > 1 {
		return Newf(ErrInvalidArgument, "ceding_commission_rate must be in [0,1]")
	}
	if c.BrokerRate < 0 || c.BrokerRate > 1 {
		return Newf(ErrInvalidArgument, "broker_rate must be in [0,1]")
	}
	if c.CededPremiumRate < 0 {
		return Newf(ErrInvalidArgument, "ceded_premium_rate must be >= 0")
	}
	if !c.EndDate.After(c.StartDate) {
		return Newf(ErrInvalidArgument, "contract end_date must be after start_date")
	}
	return nil
}

// InForceAt reports whether the contract was in force at t (inclusive of both
// endpoints) AND not cancelled.
func (c *Contract) InForceAt(t time.Time) bool {
	if c.Status == ContractCancelled {
		return false
	}
	if c.Status == ContractExpired {
		// An explicitly expired contract is no longer in force even within the
		// date window; the explicit transition overrides the calendar.
		return false
	}
	return !t.Before(c.StartDate) && !t.After(c.EndDate)
}

// SurplusCessionRate computes the per-policy cession rate for a surplus_share
// contract given the policy sum insured. Lines ceded = clamp(round((SI -
// retainedLine)/retainedLine), 0, capacity). If SI <= retainedLine the policy
// is fully retained (0 lines). cessionRate = linesCeded / (1 + linesCeded).
func (c *Contract) SurplusCessionRate(sumInsured Money) float64 {
	if c.RetainedLine <= 0 || sumInsured <= c.RetainedLine {
		return 0
	}
	linesFloat := float64(sumInsured-c.RetainedLine) / float64(c.RetainedLine)
	linesCeded := int(roundDiv(int64(sumInsured-c.RetainedLine), int64(c.RetainedLine)))
	if linesCeded < 0 {
		linesCeded = 0
	}
	if linesCeded > c.TreatyCapacity {
		linesCeded = c.TreatyCapacity
	}
	_ = linesFloat
	if linesCeded <= 0 {
		return 0
	}
	return float64(linesCeded) / float64(1+linesCeded)
}
