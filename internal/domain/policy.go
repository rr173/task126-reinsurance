package domain

import "time"

// LossStatus is the lifecycle state of a claim.
type LossStatus string

const (
	LossOpen      LossStatus = "open"
	LossAllocated LossStatus = "allocated"
	LossClosed    LossStatus = "closed"
)

// Valid reports whether s is a known loss status.
func (s LossStatus) Valid() bool {
	switch s {
	case LossOpen, LossAllocated, LossClosed:
		return true
	}
	return false
}

// Policy is a direct (original) insurance policy underwritten by the cedant and
// ceded to a treaty.
type Policy struct {
	ID              int64
	PolicyNo        string
	ContractID      int64
	SumInsured      Money
	OriginalPremium Money
	RegionTag       string
	StartDate       time.Time
	EndDate         time.Time
	// CessionRate is the proportion of premium/loss ceded. For quota_share it
	// mirrors the contract's cession_rate; for surplus_share it is computed per
	// policy from the sum insured at registration time.
	CessionRate float64
	CreatedAt   time.Time
}

// Loss is a paid claim against a policy.
type Loss struct {
	ID             int64
	ClaimNo        string
	PolicyID       int64
	ContractID     int64
	OccurrenceDate time.Time
	PaidAmount     Money
	EventTag       string // groups losses for cat_xl accumulation; "" = standalone
	Status         LossStatus
	CreatedAt      time.Time
}

// AllocationKind classifies how a recovery was computed.
type AllocationKind string

const (
	AllocationProportional AllocationKind = "proportional"
	AllocationExcessLayer  AllocationKind = "excess_layer"
	AllocationCatLayer     AllocationKind = "cat_layer"
)

// Allocation is one layer of recovery for a loss (or accumulated event).
type Allocation struct {
	ID                  int64
	LossID              int64
	ContractID          int64
	ContractType        ContractType
	Reinsurer           string
	RecoveredAmount     Money
	AttachmentConsumed  Money // portion of the loss absorbed by attachment
	LimitConsumed       Money // portion absorbed by the contract limit
	Kind                AllocationKind
	CreatedAt           time.Time
}
