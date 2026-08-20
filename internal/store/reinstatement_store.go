package store

import (
	"context"
	"time"

	"task126-reinsurance/internal/domain"
)

// ReinstatementStore provides CRUD for reinstatement records.
type ReinstatementStore struct{}

// NewReinstatementStore returns a ReinstatementStore.
func NewReinstatementStore() ReinstatementStore { return ReinstatementStore{} }

// Create inserts a reinstatement record within the caller's transaction.
func (ReinstatementStore) Create(ctx context.Context, q DBTX, r *domain.Reinstatement) error {
	now := time.Now().UTC()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	res, err := q.ExecContext(ctx, `
INSERT INTO reinstatements(contract_id,loss_id,seq,restored_amount,premium,reinstatement_factor,original_premium,created_at)
VALUES(?,?,?,?,?,?,?,?)`,
		r.ContractID, r.LossID, r.Seq, int64(r.RestoredAmount), int64(r.Premium),
		r.ReinstatementFactor, int64(r.OriginalPremium), ts(r.CreatedAt))
	if err != nil {
		return mapErr(err, "reinstatement insert")
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	r.ID = id
	return nil
}

// ListByContract returns reinstatements for a contract ordered by seq.
func (ReinstatementStore) ListByContract(ctx context.Context, q DBTX, contractID int64) ([]*domain.Reinstatement, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id,contract_id,loss_id,seq,restored_amount,premium,reinstatement_factor,original_premium,created_at
         FROM reinstatements WHERE contract_id=? ORDER BY seq`, contractID)
	if err != nil {
		return nil, mapErr(err, "reinstatement list")
	}
	defer rows.Close()
	var out []*domain.Reinstatement
	for rows.Next() {
		r := &domain.Reinstatement{}
		var createdStr string
		if err := rows.Scan(
			&r.ID, &r.ContractID, &r.LossID, &r.Seq, &r.RestoredAmount, &r.Premium,
			&r.ReinstatementFactor, &r.OriginalPremium, &createdStr,
		); err != nil {
			return nil, mapErr(err, "reinstatement scan")
		}
		r.CreatedAt = parseTs(createdStr)
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountByContract returns the number of reinstatements already used.
func (ReinstatementStore) CountByContract(ctx context.Context, q DBTX, contractID int64) (int, error) {
	row := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM reinstatements WHERE contract_id=?`, contractID)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, mapErr(err, "reinstatement count")
	}
	return n, nil
}

// SumPremiumByContractInQuarter returns the reinstatement premium charged in a quarter.
// Reinstatements are attributed to the quarter of the triggering loss occurrence.
func (ReinstatementStore) SumPremiumByContractInQuarter(ctx context.Context, q DBTX, contractID int64, year, quarter int) (domain.Money, error) {
	start, end := quarterBounds(year, quarter)
	row := q.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(r.premium),0)
         FROM reinstatements r JOIN losses l ON r.loss_id=l.id
         WHERE r.contract_id=? AND l.occurrence_date>=? AND l.occurrence_date<=?`,
		contractID, start, end)
	var sum int64
	if err := row.Scan(&sum); err != nil {
		return 0, mapErr(err, "reinstatement premium sum")
	}
	return domain.Money(sum), nil
}
