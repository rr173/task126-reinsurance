package store
import (
	"context"
	"time"
)
// AuditStore writes append-only audit log records.
type AuditStore struct{}
// NewAuditStore returns an AuditStore.
func NewAuditStore() AuditStore { return AuditStore{} }
// Append inserts an audit log entry within the caller's transaction.
func (AuditStore) Append(ctx context.Context, q DBTX, action, entityType string, entityID int64, payload string) error {
	now := time.Now().UTC()
	_, err := q.ExecContext(ctx,
		`INSERT INTO audit_logs(action,entity_type,entity_id,payload_json,created_at) VALUES(?,?,?,?,?)`,
		action, entityType, entityID, payload, ts(now))
	if err != nil {
		return mapErr(err, "audit append")
	}
	return nil
}
// AuditRecord is a read model for the audit log.
type AuditRecord struct {
	ID         int64
	Action     string
	EntityType string
	EntityID   int64
	Payload    string
	CreatedAt  time.Time
}
// ListByEntity returns audit records for an entity.
func (AuditStore) ListByEntity(ctx context.Context, q DBTX, entityType string, entityID int64) ([]AuditRecord, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id,action,entity_type,entity_id,payload_json,created_at FROM audit_logs WHERE entity_type=? AND entity_id=? ORDER BY id`,
		entityType, entityID)
	if err != nil {
		return nil, mapErr(err, "audit list")
	}
	defer rows.Close()
	var out []AuditRecord
	for rows.Next() {
		var r AuditRecord
		var createdStr string
		if err := rows.Scan(&r.ID, &r.Action, &r.EntityType, &r.EntityID, &r.Payload, &createdStr); err != nil {
			return nil, mapErr(err, "audit scan")
		}
		r.CreatedAt = parseTs(createdStr)
		out = append(out, r)
	}
	return out, rows.Err()
}
