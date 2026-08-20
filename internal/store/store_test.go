package store

import (
	"context"
	"testing"
	"time"

	"task126-reinsurance/internal/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestContractCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cs := NewContractStore()
	c := &domain.Contract{
		Code: "C1", Type: domain.QuotaShare, CessionRate: 0.5,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	if err := s.InTx(ctx, func(tx DBTX) error { return cs.Create(ctx, tx, c) }); err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID == 0 {
		t.Fatal("id not set")
	}
	var got *domain.Contract
	if err := s.InTx(ctx, func(tx DBTX) (err error) {
		got, err = cs.Get(ctx, tx, c.ID)
		return err
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Code != "C1" || got.Type != domain.QuotaShare || got.Status != domain.ContractInForce {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestDuplicateContractCodeRejected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cs := NewContractStore()
	c := &domain.Contract{
		Code: "DUP", Type: domain.QuotaShare, CessionRate: 0.5,
		StartDate: janDay(2026, 1, 1), EndDate: janDay(2026, 12, 31),
	}
	if err := s.InTx(ctx, func(tx DBTX) error { return cs.Create(ctx, tx, c) }); err != nil {
		t.Fatalf("first create: %v", err)
	}
	c2 := &domain.Contract{
		Code: "DUP", Type: domain.QuotaShare, CessionRate: 0.4,
		StartDate: janDay(2026, 1, 1), EndDate: janDay(2026, 12, 31),
	}
	err := s.InTx(ctx, func(tx DBTX) error { return cs.Create(ctx, tx, c2) })
	if !domain.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestBordereauxUpsertIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	bs := NewBordereauxStore()
	// Seed a contract to satisfy FK.
	cs := NewContractStore()
	c := &domain.Contract{
		Code: "B1", Type: domain.QuotaShare, CessionRate: 0.5,
		StartDate: janDay(2026, 1, 1), EndDate: janDay(2026, 12, 31),
	}
	if err := s.InTx(ctx, func(tx DBTX) error { return cs.Create(ctx, tx, c) }); err != nil {
		t.Fatal(err)
	}
	var b1, b2 *domain.Bordereaux
	if err := s.InTx(ctx, func(tx DBTX) (err error) {
		b1, err = bs.Upsert(ctx, tx, c.ID, 2026, 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.InTx(ctx, func(tx DBTX) (err error) {
		b2, err = bs.Upsert(ctx, tx, c.ID, 2026, 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if b1.ID != b2.ID {
		t.Fatalf("idempotent upsert returned different ids: %d vs %d", b1.ID, b2.ID)
	}
}

func TestAuditAppend(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	as := NewAuditStore()
	if err := s.InTx(ctx, func(tx DBTX) error {
		return as.Append(ctx, tx, "test", "entity", 1, `{"x":1}`)
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	var recs []AuditRecord
	if err := s.InTx(ctx, func(tx DBTX) (err error) {
		recs, err = as.ListByEntity(ctx, tx, "entity", 1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Action != "test" {
		t.Fatalf("audit record mismatch: %+v", recs)
	}
}

func TestCatastropheEventSetAccumulated(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	es := NewCatastropheEventStore()
	cs := NewContractStore()
	c := &domain.Contract{
		Code: "CAT1", Type: domain.CatXL, AttachmentPoint: 500000,
		Limit: 1000000, StartDate: janDay(2026, 1, 1), EndDate: janDay(2026, 12, 31),
	}
	if err := s.InTx(ctx, func(tx DBTX) error { return cs.Create(ctx, tx, c) }); err != nil {
		t.Fatal(err)
	}
	if err := s.InTx(ctx, func(tx DBTX) error {
		return es.SetAccumulated(ctx, tx, c.ID, "storm", 700000)
	}); err != nil {
		t.Fatal(err)
	}
	var e *domain.CatastropheEvent
	if err := s.InTx(ctx, func(tx DBTX) (err error) {
		e, err = es.Get(ctx, tx, c.ID, "storm")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if e.AccumulatedLoss != 700000 {
		t.Fatalf("accumulated loss: got %d want 700000", int64(e.AccumulatedLoss))
	}
}

func janDay(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
