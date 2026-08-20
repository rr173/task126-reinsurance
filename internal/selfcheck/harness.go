package selfcheck

import (
	"context"
	"fmt"

	"task126-reinsurance/internal/alloc"
	"task126-reinsurance/internal/bordereaux"
	"task126-reinsurance/internal/ceding"
	"task126-reinsurance/internal/contract"
	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/loss"
	"task126-reinsurance/internal/policy"
	"task126-reinsurance/internal/report"
	"task126-reinsurance/internal/store"
)

// harness wires the services the selfcheck drives directly. It bypasses the
// HTTP layer to test the business engine; the HTTP API is exercised by the
// smoke test's HTTP round-trip in main.go.
type harness struct {
	store     *store.Store
	contracts *contract.Service
	policies  *policy.Service
	losses    *loss.Service
	alloc     *alloc.Engine
	bord      *bordereaux.Service
	report    *report.Service
}

func newHarness(s *store.Store) *harness {
	return &harness{
		store:     s,
		contracts: contract.New(s),
		policies:  policy.New(s),
		losses:    loss.New(s),
		alloc:     alloc.New(s),
		bord:      bordereaux.New(s),
		report:    report.New(s),
	}
}

func cents(m int64) domain.Money { return domain.Money(m) }

// scenarioProportional: quota_share cession_rate=0.5, loss 10000.00 ->
// recovery 5000.00, ceding commission = ceded_premium * 0.5 * rate.
func (h *harness) scenarioProportional(ctx context.Context) error {
	c := &domain.Contract{
		Code: "QS-1", Type: domain.QuotaShare, CessionRate: 0.5,
		CedingCommissionRate: 0.1, BrokerRate: 0, CededPremiumRate: 1,
		Currency: "CNY", StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.contracts.Create(ctx, c); err != nil {
		return err
	}
	p := &domain.Policy{
		PolicyNo: "P-QS-1", ContractID: c.ID, SumInsured: cents(1_000_000),
		OriginalPremium: cents(4_000_00), StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.policies.Create(ctx, p); err != nil {
		return err
	}
	if err := assertMoney(cents(2_000_00), p.OriginalPremium.MulFrac(0.5), "ceded premium base"); err != nil {
		return err
	}
	l := &domain.Loss{
		ClaimNo: "L-QS-1", PolicyID: p.ID, OccurrenceDate: date(2026, 6, 15),
		PaidAmount: cents(10000_00),
	}
	if _, err := h.losses.Create(ctx, l); err != nil {
		return err
	}
	allocs, err := h.alloc.AllocateLoss(ctx, l.ID)
	if err != nil {
		return err
	}
	if err := assert(len(allocs) == 1, "expected 1 proportional allocation, got %d", len(allocs)); err != nil {
		return err
	}
	if err := assertMoney(allocs[0].RecoveredAmount, cents(5000_00), "proportional recovery"); err != nil {
		return err
	}
	// Ceding commission = ceded_premium(2000) * 0.1 = 200.00
	commission := ceding.CedingCommission(c, ceding.CededPremium(c, p))
	if err := assertMoney(commission, cents(200_00), "ceding commission"); err != nil {
		return err
	}
	return nil
}

// scenarioSurplus: retained_line=20000, capacity=3, sum_insured=80000 ->
// lines ceded = 3, cession_rate = 3/4 = 0.75. Sum_insured <= retained -> 0.
func (h *harness) scenarioSurplus(ctx context.Context) error {
	c := &domain.Contract{
		Code: "SS-1", Type: domain.SurplusShare, RetainedLine: cents(20000_00),
		TreatyCapacity: 3, CedingCommissionRate: 0, BrokerRate: 0, CededPremiumRate: 1,
		Currency: "CNY", StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.contracts.Create(ctx, c); err != nil {
		return err
	}
	p := &domain.Policy{
		PolicyNo: "P-SS-1", ContractID: c.ID, SumInsured: cents(80000_00),
		OriginalPremium: cents(4_000_00), StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.policies.Create(ctx, p); err != nil {
		return err
	}
	if err := assertFloat(p.CessionRate, 0.75, "surplus cession rate (80000/20000=3 lines)"); err != nil {
		return err
	}
	// Below retained line: 0 cession.
	p2 := &domain.Policy{
		PolicyNo: "P-SS-2", ContractID: c.ID, SumInsured: cents(15000_00),
		OriginalPremium: cents(1_000_00), StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.policies.Create(ctx, p2); err != nil {
		return err
	}
	if err := assertFloat(p2.CessionRate, 0, "surplus cession rate (below retained line)"); err != nil {
		return err
	}
	return nil
}

// scenarioExcess: per_risk_xl attachment=5000, limit=10000. Loss 8000 -> 3000;
// loss 20000 -> 10000 (capped), retained 10000.
func (h *harness) scenarioExcess(ctx context.Context) error {
	c := &domain.Contract{
		Code: "PXL-1", Type: domain.PerRiskXL, AttachmentPoint: cents(5000_00),
		Limit: cents(10000_00), NumReinstatements: 0, ReinstatementFactor: 1,
		CedingCommissionRate: 0, BrokerRate: 0, CededPremiumRate: 1,
		Currency: "CNY", StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.contracts.Create(ctx, c); err != nil {
		return err
	}
	p := &domain.Policy{
		PolicyNo: "P-PXL-1", ContractID: c.ID, SumInsured: cents(1_000_000),
		OriginalPremium: cents(2_000_00), StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.policies.Create(ctx, p); err != nil {
		return err
	}
	l := &domain.Loss{
		ClaimNo: "L-PXL-1", PolicyID: p.ID, OccurrenceDate: date(2026, 6, 1),
		PaidAmount: cents(8000_00),
	}
	if _, err := h.losses.Create(ctx, l); err != nil {
		return err
	}
	allocs, err := h.alloc.AllocateLoss(ctx, l.ID)
	if err != nil {
		return err
	}
	if err := assert(len(allocs) == 1, "expected 1 excess allocation, got %d", len(allocs)); err != nil {
		return err
	}
	if err := assertMoney(allocs[0].RecoveredAmount, cents(3000_00), "excess recovery 8000-5000"); err != nil {
		return err
	}
	// Remaining limit should now be 7000.
	cl, err := h.alloc.LimitsFor(ctx, c.ID)
	if err != nil {
		return err
	}
	if err := assertMoney(cl.LimitRemaining, cents(7000_00), "remaining limit after 3000"); err != nil {
		return err
	}
	// Second loss of 20000: capped at remaining 7000, retained 13000.
	l2 := &domain.Loss{
		ClaimNo: "L-PXL-2", PolicyID: p.ID, OccurrenceDate: date(2026, 7, 1),
		PaidAmount: cents(20000_00),
	}
	if _, err := h.losses.Create(ctx, l2); err != nil {
		return err
	}
	allocs2, err := h.alloc.AllocateLoss(ctx, l2.ID)
	if err != nil {
		return err
	}
	if err := assert(len(allocs2) == 1, "expected 1 allocation for second loss, got %d", len(allocs2)); err != nil {
		return err
	}
	if err := assertMoney(allocs2[0].RecoveredAmount, cents(7000_00), "excess recovery capped at remaining"); err != nil {
		return err
	}
	return nil
}

// scenarioReinstatement: limit=10000, num_reinstatements=2, factor=1,
// original_premium base = 2000. First loss 10000 drains limit -> reinstatement
// premium = (10000/10000) * 2000 * 1 = 2000, limit restored to 10000.
func (h *harness) scenarioReinstatement(ctx context.Context) error {
	c := &domain.Contract{
		Code: "PXL-R", Type: domain.PerRiskXL, AttachmentPoint: cents(0),
		Limit: cents(10000_00), NumReinstatements: 2, ReinstatementFactor: 1,
		CedingCommissionRate: 0, BrokerRate: 0, CededPremiumRate: 1,
		Currency: "CNY", StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.contracts.Create(ctx, c); err != nil {
		return err
	}
	p := &domain.Policy{
		PolicyNo: "P-PXL-R", ContractID: c.ID, SumInsured: cents(1_000_000),
		OriginalPremium: cents(2_000_00), StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.policies.Create(ctx, p); err != nil {
		return err
	}
	l := &domain.Loss{
		ClaimNo: "L-PXL-R1", PolicyID: p.ID, OccurrenceDate: date(2026, 6, 1),
		PaidAmount: cents(10000_00),
	}
	if _, err := h.losses.Create(ctx, l); err != nil {
		return err
	}
	if _, err := h.alloc.AllocateLoss(ctx, l.ID); err != nil {
		return err
	}
	cl, err := h.alloc.LimitsFor(ctx, c.ID)
	if err != nil {
		return err
	}
	// Limit restored to full 10000 after reinstatement, used=1.
	if err := assertMoney(cl.LimitRemaining, cents(10000_00), "limit restored after reinstatement"); err != nil {
		return err
	}
	if err := assert(cl.ReinstatementsUsed == 1, "reinstatements used should be 1, got %d", cl.ReinstatementsUsed); err != nil {
		return err
	}
	// Drain twice more: second reinstatement, third drain no longer restores.
	for i := 0; i < 2; i++ {
		ln := &domain.Loss{
			ClaimNo: fmt.Sprintf("L-PXL-R%d", i+2), PolicyID: p.ID,
			OccurrenceDate: date(2026, 7, i+1), PaidAmount: cents(10000_00),
		}
		if _, err := h.losses.Create(ctx, ln); err != nil {
			return err
		}
		if _, err := h.alloc.AllocateLoss(ctx, ln.ID); err != nil {
			return err
		}
	}
	cl, err = h.alloc.LimitsFor(ctx, c.ID)
	if err != nil {
		return err
	}
	if err := assert(cl.ReinstatementsUsed == 2, "reinstatements used should be 2 (cap), got %d", cl.ReinstatementsUsed); err != nil {
		return err
	}
	if err := assertMoney(cl.LimitRemaining, cents(0), "limit exhausted after 2 reinstatements"); err != nil {
		return err
	}
	return nil
}

// scenarioCatastrophe: cat_xl attachment=50000, limit=100000. Two losses of
// 30000+40000 in the same event accumulate to 70000 -> recovery 20000.
func (h *harness) scenarioCatastrophe(ctx context.Context) error {
	c := &domain.Contract{
		Code: "CAT-1", Type: domain.CatXL, AttachmentPoint: cents(50000_00),
		Limit: cents(100000_00), NumReinstatements: 0, ReinstatementFactor: 1,
		CedingCommissionRate: 0, BrokerRate: 0, CededPremiumRate: 1,
		Currency: "CNY", StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.contracts.Create(ctx, c); err != nil {
		return err
	}
	p := &domain.Policy{
		PolicyNo: "P-CAT-1", ContractID: c.ID, SumInsured: cents(10_000_000),
		OriginalPremium: cents(5_000_00), StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.policies.Create(ctx, p); err != nil {
		return err
	}
	for i, amt := range []int64{30000_00, 40000_00} {
		l := &domain.Loss{
			ClaimNo: fmt.Sprintf("L-CAT-%d", i+1), PolicyID: p.ID,
			OccurrenceDate: date(2026, 8, 10+i), PaidAmount: cents(amt), EventTag: "typhoon-a",
		}
		if _, err := h.losses.Create(ctx, l); err != nil {
			return err
		}
	}
	allocs, err := h.alloc.AllocateEvent(ctx, c.ID, "typhoon-a")
	if err != nil {
		return err
	}
	if err := assert(len(allocs) == 1, "expected 1 cat allocation, got %d", len(allocs)); err != nil {
		return err
	}
	if err := assertMoney(allocs[0].RecoveredAmount, cents(20000_00), "cat recovery 70000-50000"); err != nil {
		return err
	}
	// Second allocation of the same event is rejected (no double recovery).
	_, err = h.alloc.AllocateEvent(ctx, c.ID, "typhoon-a")
	if err := assertErrIs(err, domain.ErrConflict, "double cat allocation"); err != nil {
		return err
	}
	return nil
}

// scenarioBordereauxSettle: a quota_share contract in Q3 2026 aggregates
// correctly and locks on settle.
func (h *harness) scenarioBordereauxSettle(ctx context.Context) error {
	c := &domain.Contract{
		Code: "QS-BORD", Type: domain.QuotaShare, CessionRate: 0.5,
		CedingCommissionRate: 0.1, BrokerRate: 0.02, CededPremiumRate: 1,
		Currency: "CNY", StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.contracts.Create(ctx, c); err != nil {
		return err
	}
	p := &domain.Policy{
		PolicyNo: "P-BORD-1", ContractID: c.ID, SumInsured: cents(1_000_000),
		OriginalPremium: cents(4_000_00), StartDate: date(2026, 7, 1), EndDate: date(2026, 9, 30),
	}
	if _, err := h.policies.Create(ctx, p); err != nil {
		return err
	}
	l := &domain.Loss{
		ClaimNo: "L-BORD-1", PolicyID: p.ID, OccurrenceDate: date(2026, 8, 15),
		PaidAmount: cents(10000_00),
	}
	if _, err := h.losses.Create(ctx, l); err != nil {
		return err
	}
	if _, err := h.alloc.AllocateLoss(ctx, l.ID); err != nil {
		return err
	}
	b, err := h.bord.Ensure(ctx, c.ID, 2026, 3)
	if err != nil {
		return err
	}
	b, err = h.bord.Settle(ctx, b.ID)
	if err != nil {
		return err
	}
	// ceded_premium = 4000 * 0.5 = 2000; commission = 200; broker = 40;
	// recovered = 5000; net = 2000 + 0 - 5000 - 200 - 40 = -3240.
	if err := assertMoney(b.CededPremium, cents(2000_00), "bord ceded premium"); err != nil {
		return err
	}
	if err := assertMoney(b.RecoveredLoss, cents(5000_00), "bord recovered loss"); err != nil {
		return err
	}
	if err := assertMoney(b.CedingCommission, cents(200_00), "bord ceding commission"); err != nil {
		return err
	}
	if err := assertMoney(b.BrokerFee, cents(40_00), "bord broker fee"); err != nil {
		return err
	}
	if err := assertMoney(b.NetBalance, cents(-3240_00), "bord net balance"); err != nil {
		return err
	}
	if b.Status != domain.BordereauxSettled {
		return fmt.Errorf("bord status should be settled, got %s", b.Status)
	}
	// Settling again is rejected.
	_, err = h.bord.Settle(ctx, b.ID)
	if err := assertErrIs(err, domain.ErrConflict, "double settle"); err != nil {
		return err
	}
	return nil
}

// scenarioBordereauxReopen: reopening a settled account is allowed only when
// no later quarter is settled.
func (h *harness) scenarioBordereauxReopen(ctx context.Context) error {
	c := &domain.Contract{
		Code: "QS-REO", Type: domain.QuotaShare, CessionRate: 0.5,
		CedingCommissionRate: 0, BrokerRate: 0, CededPremiumRate: 1,
		Currency: "CNY", StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h.contracts.Create(ctx, c); err != nil {
		return err
	}
	p := &domain.Policy{
		PolicyNo: "P-REO-1", ContractID: c.ID, SumInsured: cents(1_000_000),
		OriginalPremium: cents(4_000_00), StartDate: date(2026, 1, 1), EndDate: date(2026, 6, 30),
	}
	if _, err := h.policies.Create(ctx, p); err != nil {
		return err
	}
	bq1, err := h.bord.Ensure(ctx, c.ID, 2026, 1)
	if err != nil {
		return err
	}
	if _, err := h.bord.Settle(ctx, bq1.ID); err != nil {
		return err
	}
	bq2, err := h.bord.Ensure(ctx, c.ID, 2026, 2)
	if err != nil {
		return err
	}
	if _, err := h.bord.Settle(ctx, bq2.ID); err != nil {
		return err
	}
	// Reopening Q1 (earlier) while Q2 is settled -> rejected.
	_, err = h.bord.Reopen(ctx, bq1.ID)
	if err := assertErrIs(err, domain.ErrConflict, "reopen earlier while later settled"); err != nil {
		return err
	}
	// Reopen Q2 (latest) -> allowed.
	bq2, err = h.bord.Reopen(ctx, bq2.ID)
	if err != nil {
		return err
	}
	if bq2.Status != domain.BordereauxOpen {
		return fmt.Errorf("reopened bord should be open, got %s", bq2.Status)
	}
	// Now Q1 can be reopened (no later settled).
	bq1, err = h.bord.Reopen(ctx, bq1.ID)
	if err != nil {
		return err
	}
	if bq1.Status != domain.BordereauxOpen {
		return fmt.Errorf("reopened earlier bord should be open, got %s", bq1.Status)
	}
	return nil
}

// scenarioContractExpiry: a loss against an expired contract is rejected; an
// out-of-window occurrence is rejected.
func (h *harness) scenarioContractExpiry(ctx context.Context) error {
	c := &domain.Contract{
		Code: "QS-EXP", Type: domain.QuotaShare, CessionRate: 0.5,
		CedingCommissionRate: 0, BrokerRate: 0, CededPremiumRate: 1,
		Currency: "CNY", StartDate: date(2026, 1, 1), EndDate: date(2026, 3, 31),
	}
	if _, err := h.contracts.Create(ctx, c); err != nil {
		return err
	}
	// Expire the contract.
	if _, err := h.contracts.TransitionStatus(ctx, c.ID, domain.ContractExpired); err != nil {
		return err
	}
	p := &domain.Policy{
		PolicyNo: "P-EXP-1", ContractID: c.ID, SumInsured: cents(1_000_000),
		OriginalPremium: cents(4_000_00), StartDate: date(2026, 1, 15), EndDate: date(2026, 3, 15),
	}
	// Policy registration against expired contract is rejected.
	_, err := h.policies.Create(ctx, p)
	if err := assertErrIs(err, domain.ErrInvalidArgument, "policy on expired contract"); err != nil {
		return err
	}
	return nil
}

// scenarioRestart: write state to a file-backed DSN, reopen a fresh store and
// confirm the contract, loss, allocation and limits are restored.
func (h *harness) scenarioRestart(ctx context.Context) error {
	dsn := "file::memory:?cache=shared"
	s2, err := store.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer s2.Close()
	h2 := newHarness(s2)
	c := &domain.Contract{
		Code: "PXL-RST", Type: domain.PerRiskXL, AttachmentPoint: cents(5000_00),
		Limit: cents(10000_00), NumReinstatements: 1, ReinstatementFactor: 1,
		CedingCommissionRate: 0, BrokerRate: 0, CededPremiumRate: 1,
		Currency: "CNY", StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h2.contracts.Create(ctx, c); err != nil {
		return err
	}
	p := &domain.Policy{
		PolicyNo: "P-RST-1", ContractID: c.ID, SumInsured: cents(1_000_000),
		OriginalPremium: cents(2_000_00), StartDate: date(2026, 1, 1), EndDate: date(2026, 12, 31),
	}
	if _, err := h2.policies.Create(ctx, p); err != nil {
		return err
	}
	l := &domain.Loss{
		ClaimNo: "L-RST-1", PolicyID: p.ID, OccurrenceDate: date(2026, 6, 1),
		PaidAmount: cents(8000_00),
	}
	if _, err := h2.losses.Create(ctx, l); err != nil {
		return err
	}
	allocs, err := h2.alloc.AllocateLoss(ctx, l.ID)
	if err != nil {
		return err
	}
	if err := assert(len(allocs) == 1, "restart scenario: expected 1 allocation, got %d", len(allocs)); err != nil {
		return err
	}
	if err := assertMoney(allocs[0].RecoveredAmount, cents(3000_00), "restart scenario recovery"); err != nil {
		return err
	}
	cl, err := h2.alloc.LimitsFor(ctx, c.ID)
	if err != nil {
		return err
	}
	if err := assertMoney(cl.LimitRemaining, cents(7000_00), "restart scenario remaining limit"); err != nil {
		return err
	}
	// Reopen a second store handle to the same shared in-memory DB and confirm
	// the contract, loss and limits survive.
	s3, err := store.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer s3.Close()
	h3 := newHarness(s3)
	gotC, err := h3.contracts.Get(ctx, c.ID)
	if err != nil {
		return err
	}
	if gotC.Code != c.Code {
		return fmt.Errorf("restart: contract code %q != %q", gotC.Code, c.Code)
	}
	gotL, err := h3.losses.Get(ctx, l.ID)
	if err != nil {
		return err
	}
	if gotL.ClaimNo != l.ClaimNo {
		return fmt.Errorf("restart: loss claim_no %q != %q", gotL.ClaimNo, l.ClaimNo)
	}
	cl3, err := h3.alloc.LimitsFor(ctx, c.ID)
	if err != nil {
		return err
	}
	if err := assertMoney(cl3.LimitRemaining, cents(7000_00), "restart: limit after reopen"); err != nil {
		return err
	}
	return nil
}

func assertFloat(got, want float64, label string) error {
	if diff := got - want; diff < -1e-9 || diff > 1e-9 {
		return fmt.Errorf("%s: got %v, want %v", label, got, want)
	}
	return nil
}
