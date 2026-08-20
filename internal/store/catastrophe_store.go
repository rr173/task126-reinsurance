package store

import (
	"context"
	"time"

	"task126-reinsurance/internal/domain"
)

// CatastropheEventStore provides access to accumulated cat events.
type CatastropheEventStore struct{}

// NewCatastropheEventStore returns a CatastropheEventStore.
func NewCatastropheEventStore() CatastropheEventStore { return CatastropheEventStore{} }

// UpsertAccumulated adds amount to the accumulated loss for a (contract, event_tag) pair.
func (CatastropheEventStore) UpsertAccumulated(ctx context.Context, q DBTX, contractID int64, eventTag string, amount domain.Money) error {
	now := time.Now().UTC()
	res, err := q.ExecContext(ctx, `
INSERT INTO catastrophe_events(contract_id,event_tag,accumulated_loss,allocated,created_at)
VALUES(?,?,?,0,?)
ON CONFLICT(contract_id,event_tag) DO UPDATE SET accumulated_loss=accumulated_loss+excluded.accumulated_loss`,
		contractID, eventTag, int64(amount), ts(now))
	if err != nil {
		return mapErr(err, "cat event upsert")
	}
	_ = res
	return nil
}

// SetAccumulated upserts the accumulated loss to exactly amount (set, not
// add). It is the authoritative reconciliation used by AllocateEvent so the
// stored event matches the sum of paid losses.
func (CatastropheEventStore) SetAccumulated(ctx context.Context, q DBTX, contractID int64, eventTag string, amount domain.Money) error {
	now := time.Now().UTC()
	res, err := q.ExecContext(ctx, `
INSERT INTO catastrophe_events(contract_id,event_tag,accumulated_loss,allocated,created_at)
VALUES(?,?,?,0,?)
ON CONFLICT(contract_id,event_tag) DO UPDATE SET accumulated_loss=excluded.accumulated_loss`,
		contractID, eventTag, int64(amount), ts(now))
	if err != nil {
		return mapErr(err, "cat event set")
	}
	_ = res
	return nil
}

// Get returns the accumulated event for a (contract, event_tag) pair.
func (CatastropheEventStore) Get(ctx context.Context, q DBTX, contractID int64, eventTag string) (*domain.CatastropheEvent, error) {
	row := q.QueryRowContext(ctx,
		`SELECT id,contract_id,event_tag,accumulated_loss,allocated,created_at
         FROM catastrophe_events WHERE contract_id=? AND event_tag=?`, contractID, eventTag)
	e := &domain.CatastropheEvent{}
	var createdStr string
	var allocatedInt int
	if err := row.Scan(&e.ID, &e.ContractID, &e.EventTag, &e.AccumulatedLoss, &allocatedInt, &createdStr); err != nil {
		return nil, mapErr(err, "cat event get")
	}
	e.Allocated = allocatedInt != 0
	e.CreatedAt = parseTs(createdStr)
	return e, nil
}

// ListByContract returns all cat events for a contract.
func (CatastropheEventStore) ListByContract(ctx context.Context, q DBTX, contractID int64) ([]*domain.CatastropheEvent, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id,contract_id,event_tag,accumulated_loss,allocated,created_at
         FROM catastrophe_events WHERE contract_id=? ORDER BY id`, contractID)
	if err != nil {
		return nil, mapErr(err, "cat event list")
	}
	defer rows.Close()
	var out []*domain.CatastropheEvent
	for rows.Next() {
		e := &domain.CatastropheEvent{}
		var createdStr string
		var allocatedInt int
		if err := rows.Scan(&e.ID, &e.ContractID, &e.EventTag, &e.AccumulatedLoss, &allocatedInt, &createdStr); err != nil {
			return nil, mapErr(err, "cat event scan")
		}
		e.Allocated = allocatedInt != 0
		e.CreatedAt = parseTs(createdStr)
		out = append(out, e)
	}
	return out, rows.Err()
}

// MarkAllocated marks a cat event as allocated so it is not re-accumulated.
func (CatastropheEventStore) MarkAllocated(ctx context.Context, q DBTX, contractID int64, eventTag string) error {
	res, err := q.ExecContext(ctx,
		`UPDATE catastrophe_events SET allocated=1 WHERE contract_id=? AND event_tag=?`,
		contractID, eventTag)
	if err != nil {
		return mapErr(err, "cat event mark")
	}
	return ensureRowsAffected(res, 1, "cat event")
}
