package store

import (
	"context"
	"time"

	"task126-reinsurance/internal/domain"
)

// LossStore provides CRUD for claims.
type LossStore struct{}

// NewLossStore returns a LossStore.
func NewLossStore() LossStore { return LossStore{} }

// Create inserts a loss within the caller's transaction.
func (LossStore) Create(ctx context.Context, q DBTX, l *domain.Loss) error {
	now := time.Now().UTC()
	if l.CreatedAt.IsZero() {
		l.CreatedAt = now
	}
	if l.Status == "" {
		l.Status = domain.LossOpen
	}
	res, err := q.ExecContext(ctx, `
INSERT INTO losses(claim_no,policy_id,contract_id,occurrence_date,paid_amount,event_tag,status,created_at)
VALUES(?,?,?,?,?,?,?,?)`,
		l.ClaimNo, l.PolicyID, l.ContractID, contractDate(l.OccurrenceDate),
		int64(l.PaidAmount), l.EventTag, string(l.Status), ts(l.CreatedAt))
	if err != nil {
		return mapErr(err, "loss insert")
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	l.ID = id
	return nil
}

func scanLossRow(scan func(dest ...any) error) (*domain.Loss, error) {
	l := &domain.Loss{}
	var occStr, createdStr, statusStr string
	if err := scan(
		&l.ID, &l.ClaimNo, &l.PolicyID, &l.ContractID, &occStr, &l.PaidAmount, &l.EventTag, &statusStr, &createdStr,
	); err != nil {
		return nil, mapErr(err, "loss scan")
	}
	l.OccurrenceDate = parseDate(occStr)
	l.Status = domain.LossStatus(statusStr)
	l.CreatedAt = parseTs(createdStr)
	return l, nil
}

const lossSelectCols = `id,claim_no,policy_id,contract_id,occurrence_date,paid_amount,event_tag,status,created_at`

// Get fetches a loss by id.
func (s LossStore) Get(ctx context.Context, q DBTX, id int64) (*domain.Loss, error) {
	row := q.QueryRowContext(ctx, `SELECT `+lossSelectCols+` FROM losses WHERE id=?`, id)
	return scanLossRow(row.Scan)
}

// GetByClaimNo fetches a loss by claim number.
func (s LossStore) GetByClaimNo(ctx context.Context, q DBTX, claimNo string) (*domain.Loss, error) {
	row := q.QueryRowContext(ctx, `SELECT id FROM losses WHERE claim_no=?`, claimNo)
	var id int64
	if err := row.Scan(&id); err != nil {
		return nil, mapErr(err, "loss by claim_no")
	}
	return s.Get(ctx, q, id)
}

// List returns all losses ordered by id.
func (s LossStore) List(ctx context.Context, q DBTX) ([]*domain.Loss, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+lossSelectCols+` FROM losses ORDER BY id`)
	if err != nil {
		return nil, mapErr(err, "loss list")
	}
	defer rows.Close()
	var out []*domain.Loss
	for rows.Next() {
		l, err := scanLossRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ListByContractEvent returns losses for a (contract, event_tag) pair.
func (s LossStore) ListByContractEvent(ctx context.Context, q DBTX, contractID int64, eventTag string) ([]*domain.Loss, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+lossSelectCols+` FROM losses WHERE contract_id=? AND event_tag=? ORDER BY id`, contractID, eventTag)
	if err != nil {
		return nil, mapErr(err, "loss list by event")
	}
	defer rows.Close()
	var out []*domain.Loss
	for rows.Next() {
		l, err := scanLossRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ListByContract returns losses for a contract.
func (s LossStore) ListByContract(ctx context.Context, q DBTX, contractID int64) ([]*domain.Loss, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+lossSelectCols+` FROM losses WHERE contract_id=? ORDER BY id`, contractID)
	if err != nil {
		return nil, mapErr(err, "loss list by contract")
	}
	defer rows.Close()
	var out []*domain.Loss
	for rows.Next() {
		l, err := scanLossRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// UpdateStatus sets the loss lifecycle status.
func (LossStore) UpdateStatus(ctx context.Context, q DBTX, id int64, status domain.LossStatus) error {
	res, err := q.ExecContext(ctx, `UPDATE losses SET status=? WHERE id=?`, string(status), id)
	if err != nil {
		return mapErr(err, "loss status update")
	}
	return ensureRowsAffected(res, 1, "loss")
}

// SumPaidByContractEvent returns the sum of paid_amount for a (contract, event_tag) pair.
func (LossStore) SumPaidByContractEvent(ctx context.Context, q DBTX, contractID int64, eventTag string) (domain.Money, error) {
	row := q.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(paid_amount),0) FROM losses WHERE contract_id=? AND event_tag=?`, contractID, eventTag)
	var sum int64
	if err := row.Scan(&sum); err != nil {
		return 0, mapErr(err, "loss sum by event")
	}
	return domain.Money(sum), nil
}
