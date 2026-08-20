package domain

import "time"

// Reinstatement records one reinstatement of an excess contract's limit after
// a loss exhausted it.
type Reinstatement struct {
	ID                 int64
	ContractID         int64
	LossID             int64
	Seq                int     // 1-based sequence within the contract
	RestoredAmount     Money   // amount of limit restored (== limit, for full reinstatements)
	Premium            Money   // reinstatement premium charged
	ReinstatementFactor float64
	OriginalPremium    Money
	CreatedAt          time.Time
}

// CatastropheEvent is the accumulated loss for a (contract, event_tag) pair
// under a cat_xl treaty. It is the input to the cat allocation step.
type CatastropheEvent struct {
	ID              int64
	ContractID      int64
	EventTag        string
	AccumulatedLoss Money
	Allocated       bool
	CreatedAt       time.Time
}

// ContractLimit is the authoritative running state of an excess contract:
// the remaining limit available and the number of reinstatements already used.
type ContractLimit struct {
	ContractID         int64
	LimitRemaining     Money
	ReinstatementsUsed int
	UpdatedAt          time.Time
}

// BordereauxStatus is the lifecycle state of a quarterly account.
type BordereauxStatus string

const (
	BordereauxOpen    BordereauxStatus = "open"
	BordereauxSettled BordereauxStatus = "settled"
)

// Bordereaux is a quarterly account for a contract summarising ceded premium,
// recoveries, commissions and reinstatement premiums.
type Bordereaux struct {
	ID                   int64
	ContractID           int64
	PeriodYear           int
	PeriodQuarter        int // 1..4
	CededPremium         Money
	RecoveredLoss        Money
	CedingCommission     Money
	BrokerFee            Money
	ReinstatementPremium Money
	NetBalance           Money
	Status               BordereauxStatus
	SettledAt            *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AuditLog is an append-only record of mutating operations.
type AuditLog struct {
	ID         int64
	Action     string
	EntityType string
	EntityID   int64
	Payload    string // JSON
	CreatedAt  time.Time
}

// QuarterOf returns the calendar quarter (1..4) for t in the engine's local
// reckoning (UTC). It is used to bucket bordereaux.
func QuarterOf(t time.Time) (year int, quarter int) {
	y := t.UTC().Year()
	q := (int(t.UTC().Month())-1)/3 + 1
	return y, q
}
