package store

import (
	"context"
	"time"

	"task126-reinsurance/internal/domain"
)

// AllocationStore provides CRUD for recovery layers.
type AllocationStore struct{}

// AllocationAttribution assigns a share of a single catastrophe allocation to
// one of the losses that formed its event total.  The parent allocation remains
// the authoritative contract-level ledger entry and is therefore not copied.
type AllocationAttribution struct {
	AllocationID    int64
	LossID          int64
	RecoveredAmount domain.Money
}

// NewAllocationStore returns an AllocationStore.
func NewAllocationStore() AllocationStore { return AllocationStore{} }

// Create inserts an allocation within the caller's transaction.
func (AllocationStore) Create(ctx context.Context, q DBTX, a *domain.Allocation) error {
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	res, err := q.ExecContext(ctx, `
INSERT INTO allocations(loss_id,contract_id,contract_type,reinsurer,recovered_amount,attachment_consumed,limit_consumed,kind,created_at)
VALUES(?,?,?,?,?,?,?,?,?)`,
		a.LossID, a.ContractID, string(a.ContractType), a.Reinsurer, int64(a.RecoveredAmount),
		int64(a.AttachmentConsumed), int64(a.LimitConsumed), string(a.Kind), ts(a.CreatedAt))
	if err != nil {
		return mapErr(err, "allocation insert")
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	a.ID = id
	return nil
}

// CreateAttributions persists the per-loss shares of an event-level cat layer.
// It is called in the same transaction as the allocation that it references.
func (AllocationStore) CreateAttributions(ctx context.Context, q DBTX, attributions []AllocationAttribution) error {
	for _, attribution := range attributions {
		if _, err := q.ExecContext(ctx, `
INSERT INTO allocation_attributions(allocation_id,loss_id,recovered_amount) VALUES(?,?,?)`,
			attribution.AllocationID, attribution.LossID, int64(attribution.RecoveredAmount)); err != nil {
			return mapErr(err, "allocation attribution insert")
		}
	}
	return nil
}

func scanAllocationRow(scan func(dest ...any) error) (*domain.Allocation, error) {
	a := &domain.Allocation{}
	var typeStr, kindStr, createdStr string
	if err := scan(
		&a.ID, &a.LossID, &a.ContractID, &typeStr, &a.Reinsurer, &a.RecoveredAmount,
		&a.AttachmentConsumed, &a.LimitConsumed, &kindStr, &createdStr,
	); err != nil {
		return nil, mapErr(err, "allocation scan")
	}
	a.ContractType = domain.ContractType(typeStr)
	a.Kind = domain.AllocationKind(kindStr)
	a.CreatedAt = parseTs(createdStr)
	return a, nil
}

const allocationSelectCols = `id,loss_id,contract_id,contract_type,reinsurer,recovered_amount,attachment_consumed,limit_consumed,kind,created_at`

// ListByLoss returns allocations for a loss.
func (AllocationStore) ListByLoss(ctx context.Context, q DBTX, lossID int64) ([]*domain.Allocation, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+allocationSelectCols+` FROM allocations WHERE loss_id=? ORDER BY id`, lossID)
	if err != nil {
		return nil, mapErr(err, "allocation list by loss")
	}
	defer rows.Close()
	var out []*domain.Allocation
	for rows.Next() {
		a, err := scanAllocationRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SumRecoveredByLoss returns the recovery economically attributable to one
// loss.  Ordinary layers are stored directly on the loss; cat layers are
// stored once per event and contribute through allocation_attributions.
func (AllocationStore) SumRecoveredByLoss(ctx context.Context, q DBTX, lossID int64) (domain.Money, error) {
	row := q.QueryRowContext(ctx, `
SELECT
  COALESCE((SELECT SUM(recovered_amount) FROM allocations WHERE loss_id=? AND kind<>?), 0) +
  COALESCE((SELECT SUM(aa.recovered_amount)
            FROM allocation_attributions aa
            JOIN allocations a ON a.id=aa.allocation_id
            WHERE aa.loss_id=? AND a.kind=?), 0)`,
		lossID, string(domain.AllocationCatLayer), lossID, string(domain.AllocationCatLayer))
	var sum int64
	if err := row.Scan(&sum); err != nil {
		return 0, mapErr(err, "allocation sum by loss")
	}
	return domain.Money(sum), nil
}

// SumRecoveredByContract returns total recovered amount for a contract.
func (AllocationStore) SumRecoveredByContract(ctx context.Context, q DBTX, contractID int64) (domain.Money, error) {
	row := q.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(recovered_amount),0) FROM allocations WHERE contract_id=?`, contractID)
	var sum int64
	if err := row.Scan(&sum); err != nil {
		return 0, mapErr(err, "allocation sum by contract")
	}
	return domain.Money(sum), nil
}

// SumRecoveredByContractInQuarter returns recovered amounts in a quarter.
func (AllocationStore) SumRecoveredByContractInQuarter(ctx context.Context, q DBTX, contractID int64, year, quarter int) (domain.Money, error) {
	start, end := quarterBounds(year, quarter)
	row := q.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(a.recovered_amount),0)
         FROM allocations a JOIN losses l ON a.loss_id=l.id
         WHERE a.contract_id=? AND l.occurrence_date>=? AND l.occurrence_date<=?`,
		contractID, start, end)
	var sum int64
	if err := row.Scan(&sum); err != nil {
		return 0, mapErr(err, "allocation sum by contract quarter")
	}
	return domain.Money(sum), nil
}

// quarterBounds returns YYYY-MM-DD strings for the inclusive quarter date range.
func quarterBounds(year, quarter int) (string, string) {
	q := ((quarter-1)%4 + 4) % 4 // normalise 1..4
	startMonth := q*3 + 1
	endMonth := startMonth + 2
	start := firstDayOfMonth(year, startMonth)
	end := firstDayOfMonth(year, endMonth).AddDate(0, 1, -1)
	return contractDate(start), contractDate(end)
}

func firstDayOfMonth(year, month int) time.Time {
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
}
