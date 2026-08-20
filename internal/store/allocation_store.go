package store

import (
	"context"
	"time"

	"task126-reinsurance/internal/domain"
)

// AllocationStore provides CRUD for recovery layers.
type AllocationStore struct{}

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
	q := ((quarter - 1) % 4 + 4) % 4 // normalise 1..4
	startMonth := q*3 + 1
	endMonth := startMonth + 2
	start := firstDayOfMonth(year, startMonth)
	end := firstDayOfMonth(year, endMonth).AddDate(0, 1, -1)
	return contractDate(start), contractDate(end)
}

func firstDayOfMonth(year, month int) time.Time {
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
}
