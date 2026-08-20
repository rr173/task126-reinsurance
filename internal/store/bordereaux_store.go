package store

import (
	"context"
	"database/sql"
	"time"

	"task126-reinsurance/internal/domain"
)

// BordereauxAggregates holds the computed totals written to a bordereaux at
// settlement time.
type BordereauxAggregates struct {
	CededPremium         domain.Money
	RecoveredLoss        domain.Money
	CedingCommission     domain.Money
	BrokerFee            domain.Money
	ReinstatementPremium domain.Money
	NetBalance           domain.Money
}

// BordereauxStore provides CRUD for quarterly accounts.
type BordereauxStore struct{}

// NewBordereauxStore returns a BordereauxStore.
func NewBordereauxStore() BordereauxStore { return BordereauxStore{} }

// Upsert creates or returns an open bordereaux for a (contract, year, quarter)
// triple. If one already exists (any status) it is returned unchanged.
func (BordereauxStore) Upsert(ctx context.Context, q DBTX, contractID int64, year, quarter int) (*domain.Bordereaux, error) {
	now := time.Now().UTC()
	if _, err := q.ExecContext(ctx, `
INSERT INTO bordereaux(contract_id,period_year,period_quarter,ceded_premium,recovered_loss,
 ceding_commission,broker_fee,reinstatement_premium,net_balance,status,settled_at,created_at,updated_at)
VALUES(?,?,?,0,0,0,0,0,0,'open',NULL,?,?)
ON CONFLICT(contract_id,period_year,period_quarter) DO NOTHING`,
		contractID, year, quarter, ts(now), ts(now)); err != nil {
		return nil, mapErr(err, "bordereaux upsert")
	}
	return BordereauxStore{}.GetByPeriod(ctx, q, contractID, year, quarter)
}

func scanBordereauxRow(scan func(dest ...any) error) (*domain.Bordereaux, error) {
	b := &domain.Bordereaux{}
	var statusStr string
	var settledStr sql.NullString
	var createdStr, updatedStr string
	if err := scan(
		&b.ID, &b.ContractID, &b.PeriodYear, &b.PeriodQuarter, &b.CededPremium, &b.RecoveredLoss,
		&b.CedingCommission, &b.BrokerFee, &b.ReinstatementPremium, &b.NetBalance, &statusStr,
		&settledStr, &createdStr, &updatedStr,
	); err != nil {
		return nil, mapErr(err, "bordereaux scan")
	}
	b.Status = domain.BordereauxStatus(statusStr)
	if settledStr.Valid && settledStr.String != "" {
		t := parseTs(settledStr.String)
		b.SettledAt = &t
	}
	b.CreatedAt = parseTs(createdStr)
	b.UpdatedAt = parseTs(updatedStr)
	return b, nil
}

const bordereauxSelectCols = `id,contract_id,period_year,period_quarter,ceded_premium,recovered_loss,
 ceding_commission,broker_fee,reinstatement_premium,net_balance,status,settled_at,created_at,updated_at`

// Get fetches a bordereaux by id.
func (s BordereauxStore) Get(ctx context.Context, q DBTX, id int64) (*domain.Bordereaux, error) {
	row := q.QueryRowContext(ctx, `SELECT `+bordereauxSelectCols+` FROM bordereaux WHERE id=?`, id)
	return scanBordereauxRow(row.Scan)
}

// GetByPeriod fetches a bordereaux by (contract, year, quarter).
func (s BordereauxStore) GetByPeriod(ctx context.Context, q DBTX, contractID int64, year, quarter int) (*domain.Bordereaux, error) {
	row := q.QueryRowContext(ctx, `SELECT `+bordereauxSelectCols+` FROM bordereaux WHERE contract_id=? AND period_year=? AND period_quarter=?`,
		contractID, year, quarter)
	return scanBordereauxRow(row.Scan)
}

// List returns all bordereaux ordered by id.
func (s BordereauxStore) List(ctx context.Context, q DBTX) ([]*domain.Bordereaux, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+bordereauxSelectCols+` FROM bordereaux ORDER BY contract_id, period_year, period_quarter`)
	if err != nil {
		return nil, mapErr(err, "bordereaux list")
	}
	defer rows.Close()
	var out []*domain.Bordereaux
	for rows.Next() {
		b, err := scanBordereauxRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListByContract returns bordereaux for a contract.
func (s BordereauxStore) ListByContract(ctx context.Context, q DBTX, contractID int64) ([]*domain.Bordereaux, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+bordereauxSelectCols+` FROM bordereaux WHERE contract_id=? ORDER BY period_year, period_quarter`, contractID)
	if err != nil {
		return nil, mapErr(err, "bordereaux list by contract")
	}
	defer rows.Close()
	var out []*domain.Bordereaux
	for rows.Next() {
		b, err := scanBordereauxRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Settle writes the computed aggregates and marks the bordereaux settled.
func (BordereauxStore) Settle(ctx context.Context, q DBTX, id int64, agg BordereauxAggregates) error {
	now := time.Now().UTC()
	res, err := q.ExecContext(ctx, `
UPDATE bordereaux
SET ceded_premium=?,recovered_loss=?,ceding_commission=?,broker_fee=?,reinstatement_premium=?,net_balance=?,status='settled',settled_at=?,updated_at=?
WHERE id=? AND status='open'`,
		int64(agg.CededPremium), int64(agg.RecoveredLoss), int64(agg.CedingCommission),
		int64(agg.BrokerFee), int64(agg.ReinstatementPremium), int64(agg.NetBalance),
		ts(now), ts(now), id)
	if err != nil {
		return mapErr(err, "bordereaux settle")
	}
	return ensureRowsAffected(res, 1, "bordereaux settle")
}

// Reopen marks a settled bordereaux open again (caller verifies ordering).
func (BordereauxStore) Reopen(ctx context.Context, q DBTX, id int64) error {
	now := time.Now().UTC()
	res, err := q.ExecContext(ctx,
		`UPDATE bordereaux SET status='open', settled_at=NULL, updated_at=? WHERE id=? AND status='settled'`,
		ts(now), id)
	if err != nil {
		return mapErr(err, "bordereaux reopen")
	}
	return ensureRowsAffected(res, 1, "bordereaux reopen")
}

// HasLaterSettled reports whether any bordereaux for contractID is settled with
// a period strictly after (year, quarter).
func (BordereauxStore) HasLaterSettled(ctx context.Context, q DBTX, contractID int64, year, quarter int) (bool, error) {
	row := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bordereaux WHERE contract_id=? AND status='settled' AND (period_year>? OR (period_year=? AND period_quarter>?))`,
		contractID, year, year, quarter)
	var n int
	if err := row.Scan(&n); err != nil {
		return false, mapErr(err, "bordereaux later settled")
	}
	return n > 0, nil
}
