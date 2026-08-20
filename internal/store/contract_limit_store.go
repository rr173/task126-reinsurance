package store

import (
	"context"
	"time"

	"task126-reinsurance/internal/domain"
)

// ContractLimitStore provides access to the running state of excess contracts.
type ContractLimitStore struct{}

// NewContractLimitStore returns a ContractLimitStore.
func NewContractLimitStore() ContractLimitStore { return ContractLimitStore{} }

// Get fetches the running limit state for an excess contract.
func (ContractLimitStore) Get(ctx context.Context, q DBTX, contractID int64) (*domain.ContractLimit, error) {
	row := q.QueryRowContext(ctx,
		`SELECT contract_id,limit_remaining,reinstatements_used,updated_at FROM contract_limits WHERE contract_id=?`,
		contractID)
	cl := &domain.ContractLimit{}
	var updatedStr string
	if err := row.Scan(&cl.ContractID, &cl.LimitRemaining, &cl.ReinstatementsUsed, &updatedStr); err != nil {
		return nil, mapErr(err, "contract_limit get")
	}
	cl.UpdatedAt = parseTs(updatedStr)
	return cl, nil
}

// DecrementLimit reduces the remaining limit by amount within the caller's tx.
// It is the authoritative mutation during excess allocation.
func (ContractLimitStore) DecrementLimit(ctx context.Context, q DBTX, contractID int64, amount domain.Money) error {
	res, err := q.ExecContext(ctx,
		`UPDATE contract_limits SET limit_remaining=limit_remaining-?, updated_at=? WHERE contract_id=?`,
		int64(amount), ts(time.Now().UTC()), contractID)
	if err != nil {
		return mapErr(err, "contract_limit decrement")
	}
	return ensureRowsAffected(res, 1, "contract_limit")
}

// RestoreLimit refills the remaining limit to the full contract limit and bumps
// reinstatements_used. called during a reinstatement.
func (ContractLimitStore) RestoreLimit(ctx context.Context, q DBTX, contractID int64, fullLimit domain.Money) error {
	res, err := q.ExecContext(ctx,
		`UPDATE contract_limits SET limit_remaining=?, reinstatements_used=reinstatements_used+1, updated_at=? WHERE contract_id=?`,
		int64(fullLimit), ts(time.Now().UTC()), contractID)
	if err != nil {
		return mapErr(err, "contract_limit restore")
	}
	return ensureRowsAffected(res, 1, "contract_limit")
}
