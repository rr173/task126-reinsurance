package store

import (
	"context"
	"fmt"
)

const schema = `
CREATE TABLE IF NOT EXISTS contracts (
  id INTEGER PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  type TEXT NOT NULL,
  attachment_point INTEGER NOT NULL DEFAULT 0,
  limit_cents INTEGER NOT NULL DEFAULT 0,
  cession_rate REAL NOT NULL DEFAULT 0,
  retained_line INTEGER NOT NULL DEFAULT 0,
  treaty_capacity INTEGER NOT NULL DEFAULT 0,
  num_reinstatements INTEGER NOT NULL DEFAULT 0,
  reinstatement_factor REAL NOT NULL DEFAULT 0,
  ceding_commission_rate REAL NOT NULL DEFAULT 0,
  broker_rate REAL NOT NULL DEFAULT 0,
  ceded_premium_rate REAL NOT NULL DEFAULT 1.0,
  currency TEXT NOT NULL DEFAULT 'CNY',
  start_date TEXT NOT NULL,
  end_date TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'in_force',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS policies (
  id INTEGER PRIMARY KEY,
  policy_no TEXT NOT NULL UNIQUE,
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  sum_insured INTEGER NOT NULL,
  original_premium INTEGER NOT NULL,
  region_tag TEXT NOT NULL DEFAULT '',
  start_date TEXT NOT NULL,
  end_date TEXT NOT NULL,
  cession_rate REAL NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_policies_contract ON policies(contract_id);

CREATE TABLE IF NOT EXISTS losses (
  id INTEGER PRIMARY KEY,
  claim_no TEXT NOT NULL UNIQUE,
  policy_id INTEGER NOT NULL REFERENCES policies(id),
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  occurrence_date TEXT NOT NULL,
  paid_amount INTEGER NOT NULL,
  event_tag TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'open',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_losses_contract ON losses(contract_id);
CREATE INDEX IF NOT EXISTS idx_losses_policy ON losses(policy_id);
CREATE INDEX IF NOT EXISTS idx_losses_event ON losses(contract_id, event_tag);

CREATE TABLE IF NOT EXISTS allocations (
  id INTEGER PRIMARY KEY,
  loss_id INTEGER NOT NULL REFERENCES losses(id),
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  contract_type TEXT NOT NULL,
  reinsurer TEXT NOT NULL DEFAULT '',
  recovered_amount INTEGER NOT NULL,
  attachment_consumed INTEGER NOT NULL DEFAULT 0,
  limit_consumed INTEGER NOT NULL DEFAULT 0,
  kind TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_allocations_loss ON allocations(loss_id);
CREATE INDEX IF NOT EXISTS idx_allocations_contract ON allocations(contract_id);

CREATE TABLE IF NOT EXISTS reinstatements (
  id INTEGER PRIMARY KEY,
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  loss_id INTEGER NOT NULL REFERENCES losses(id),
  seq INTEGER NOT NULL,
  restored_amount INTEGER NOT NULL,
  premium INTEGER NOT NULL,
  reinstatement_factor REAL NOT NULL,
  original_premium INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(contract_id, seq)
);

CREATE TABLE IF NOT EXISTS catastrophe_events (
  id INTEGER PRIMARY KEY,
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  event_tag TEXT NOT NULL,
  accumulated_loss INTEGER NOT NULL DEFAULT 0,
  allocated INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  UNIQUE(contract_id, event_tag)
);

CREATE TABLE IF NOT EXISTS contract_limits (
  contract_id INTEGER PRIMARY KEY REFERENCES contracts(id),
  limit_remaining INTEGER NOT NULL,
  reinstatements_used INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS bordereaux (
  id INTEGER PRIMARY KEY,
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  period_year INTEGER NOT NULL,
  period_quarter INTEGER NOT NULL,
  ceded_premium INTEGER NOT NULL DEFAULT 0,
  recovered_loss INTEGER NOT NULL DEFAULT 0,
  ceding_commission INTEGER NOT NULL DEFAULT 0,
  broker_fee INTEGER NOT NULL DEFAULT 0,
  reinstatement_premium INTEGER NOT NULL DEFAULT 0,
  net_balance INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'open',
  settled_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(contract_id, period_year, period_quarter)
);
CREATE INDEX IF NOT EXISTS idx_bordereaux_contract_period ON bordereaux(contract_id, period_year, period_quarter);

CREATE TABLE IF NOT EXISTS audit_logs (
  id INTEGER PRIMARY KEY,
  action TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id INTEGER,
  payload_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_logs(entity_type, entity_id);
`

// migrate applies the schema idempotently.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	return nil
}
