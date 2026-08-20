package store

import (
	"context"
	"time"

	"task126-reinsurance/internal/domain"
)

// ContractStore provides CRUD for treaties.
type ContractStore struct{}

// NewContractStore returns a ContractStore.
func NewContractStore() ContractStore { return ContractStore{} }

// Create inserts a contract and its initial contract_limits row (for excess
// types) within the caller's transaction.
func (ContractStore) Create(ctx context.Context, q DBTX, c *domain.Contract) error {
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if c.Status == "" {
		c.Status = domain.ContractInForce
	}
	res, err := q.ExecContext(ctx, `
INSERT INTO contracts
(code,type,attachment_point,limit_cents,cession_rate,retained_line,treaty_capacity,
 num_reinstatements,reinstatement_factor,ceding_commission_rate,broker_rate,ceded_premium_rate,
 currency,start_date,end_date,status,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.Code, string(c.Type), int64(c.AttachmentPoint), int64(c.Limit), c.CessionRate,
		int64(c.RetainedLine), c.TreatyCapacity, c.NumReinstatements, c.ReinstatementFactor,
		c.CedingCommissionRate, c.BrokerRate, c.CededPremiumRate, c.Currency,
		contractDate(c.StartDate), contractDate(c.EndDate), string(c.Status),
		ts(c.CreatedAt), ts(c.UpdatedAt))
	if err != nil {
		return mapErr(err, "contract insert")
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	c.ID = id
	if c.Type.IsExcess() {
		if _, err := q.ExecContext(ctx, `
INSERT INTO contract_limits(contract_id,limit_remaining,reinstatements_used,updated_at)
VALUES(?,?,0,?)`,
			c.ID, int64(c.Limit), ts(c.UpdatedAt)); err != nil {
			return mapErr(err, "contract_limits insert")
		}
	}
	return nil
}

func scanContractRow(scan func(dest ...any) error) (*domain.Contract, error) {
	c := &domain.Contract{}
	var typeStr, statusStr, startStr, endStr, createdStr, updatedStr string
	if err := scan(
		&c.ID, &c.Code, &typeStr, &c.AttachmentPoint, &c.Limit, &c.CessionRate, &c.RetainedLine,
		&c.TreatyCapacity, &c.NumReinstatements, &c.ReinstatementFactor, &c.CedingCommissionRate,
		&c.BrokerRate, &c.CededPremiumRate, &c.Currency, &startStr, &endStr, &statusStr,
		&createdStr, &updatedStr,
	); err != nil {
		return nil, mapErr(err, "contract scan")
	}
	c.Type = domain.ContractType(typeStr)
	c.Status = domain.ContractStatus(statusStr)
	c.StartDate = parseDate(startStr)
	c.EndDate = parseDate(endStr)
	c.CreatedAt = parseTs(createdStr)
	c.UpdatedAt = parseTs(updatedStr)
	return c, nil
}

const contractSelectCols = `id,code,type,attachment_point,limit_cents,cession_rate,retained_line,treaty_capacity,
       num_reinstatements,reinstatement_factor,ceding_commission_rate,broker_rate,ceded_premium_rate,
       currency,start_date,end_date,status,created_at,updated_at`

// Get fetches a contract by id.
func (s ContractStore) Get(ctx context.Context, q DBTX, id int64) (*domain.Contract, error) {
	row := q.QueryRowContext(ctx, `SELECT `+contractSelectCols+` FROM contracts WHERE id=?`, id)
	return scanContractRow(row.Scan)
}

// GetByCode fetches a contract by its unique code.
func (s ContractStore) GetByCode(ctx context.Context, q DBTX, code string) (*domain.Contract, error) {
	row := q.QueryRowContext(ctx, `SELECT id FROM contracts WHERE code=?`, code)
	var id int64
	if err := row.Scan(&id); err != nil {
		return nil, mapErr(err, "contract by code")
	}
	return s.Get(ctx, q, id)
}

// List returns all contracts ordered by id.
func (ContractStore) List(ctx context.Context, q DBTX) ([]*domain.Contract, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+contractSelectCols+` FROM contracts ORDER BY id`)
	if err != nil {
		return nil, mapErr(err, "contract list")
	}
	defer rows.Close()
	var out []*domain.Contract
	for rows.Next() {
		c, err := scanContractRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateStatus advances a contract's lifecycle status.
func (ContractStore) UpdateStatus(ctx context.Context, q DBTX, id int64, status domain.ContractStatus) error {
	res, err := q.ExecContext(ctx,
		`UPDATE contracts SET status=?, updated_at=? WHERE id=?`,
		string(status), ts(time.Now().UTC()), id)
	if err != nil {
		return mapErr(err, "contract status update")
	}
	return ensureRowsAffected(res, 1, "contract")
}
