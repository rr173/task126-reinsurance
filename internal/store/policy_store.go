package store

import (
	"context"
	"time"

	"task126-reinsurance/internal/domain"
)

// PolicyStore provides CRUD for direct policies.
type PolicyStore struct{}

// NewPolicyStore returns a PolicyStore.
func NewPolicyStore() PolicyStore { return PolicyStore{} }

// Create inserts a policy within the caller's transaction.
func (PolicyStore) Create(ctx context.Context, q DBTX, p *domain.Policy) error {
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	res, err := q.ExecContext(ctx, `
INSERT INTO policies(policy_no,contract_id,sum_insured,original_premium,region_tag,start_date,end_date,cession_rate,created_at)
VALUES(?,?,?,?,?,?,?,?,?)`,
		p.PolicyNo, p.ContractID, int64(p.SumInsured), int64(p.OriginalPremium), p.RegionTag,
		contractDate(p.StartDate), contractDate(p.EndDate), p.CessionRate, ts(p.CreatedAt))
	if err != nil {
		return mapErr(err, "policy insert")
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

func scanPolicyRow(scan func(dest ...any) error) (*domain.Policy, error) {
	p := &domain.Policy{}
	var startStr, endStr, createdStr string
	if err := scan(
		&p.ID, &p.PolicyNo, &p.ContractID, &p.SumInsured, &p.OriginalPremium, &p.RegionTag,
		&startStr, &endStr, &p.CessionRate, &createdStr,
	); err != nil {
		return nil, mapErr(err, "policy scan")
	}
	p.StartDate = parseDate(startStr)
	p.EndDate = parseDate(endStr)
	p.CreatedAt = parseTs(createdStr)
	return p, nil
}

const policySelectCols = `id,policy_no,contract_id,sum_insured,original_premium,region_tag,start_date,end_date,cession_rate,created_at`

// Get fetches a policy by id.
func (s PolicyStore) Get(ctx context.Context, q DBTX, id int64) (*domain.Policy, error) {
	row := q.QueryRowContext(ctx, `SELECT `+policySelectCols+` FROM policies WHERE id=?`, id)
	return scanPolicyRow(row.Scan)
}

// ListByContract returns policies for a contract.
func (s PolicyStore) ListByContract(ctx context.Context, q DBTX, contractID int64) ([]*domain.Policy, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+policySelectCols+` FROM policies WHERE contract_id=? ORDER BY id`, contractID)
	if err != nil {
		return nil, mapErr(err, "policy list")
	}
	defer rows.Close()
	var out []*domain.Policy
	for rows.Next() {
		p, err := scanPolicyRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// List returns all policies ordered by id.
func (s PolicyStore) List(ctx context.Context, q DBTX) ([]*domain.Policy, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+policySelectCols+` FROM policies ORDER BY id`)
	if err != nil {
		return nil, mapErr(err, "policy list")
	}
	defer rows.Close()
	var out []*domain.Policy
	for rows.Next() {
		p, err := scanPolicyRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
