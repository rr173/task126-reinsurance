package httpapi

import "task126-reinsurance/internal/domain"

// policyDTO is the JSON shape for a policy.
type policyDTO struct {
	ID                     int64   `json:"id"`
	PolicyNo               string  `json:"policy_no"`
	ContractID             int64   `json:"contract_id"`
	SumInsuredCents        int64   `json:"sum_insured_cents"`
	OriginalPremiumCents   int64   `json:"original_premium_cents"`
	RegionTag              string  `json:"region_tag"`
	StartDate              string  `json:"start_date"`
	EndDate                string  `json:"end_date"`
	CessionRate            float64 `json:"cession_rate"`
}

func policyToDTO(p *domain.Policy) policyDTO {
	return policyDTO{
		ID: p.ID, PolicyNo: p.PolicyNo, ContractID: p.ContractID,
		SumInsuredCents: moneyToCents(p.SumInsured), OriginalPremiumCents: moneyToCents(p.OriginalPremium),
		RegionTag: p.RegionTag, StartDate: p.StartDate.Format("2006-01-02"),
		EndDate: p.EndDate.Format("2006-01-02"), CessionRate: p.CessionRate,
	}
}

type policyCreateReq struct {
	PolicyNo        string `json:"policy_no"`
	ContractID      int64  `json:"contract_id"`
	SumInsuredCents int64  `json:"sum_insured_cents"`
	OriginalPremiumCents int64 `json:"original_premium_cents"`
	RegionTag       string `json:"region_tag"`
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
}

func (r policyCreateReq) toDomain() (*domain.Policy, error) {
	start, err := parseDate(r.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := parseDate(r.EndDate)
	if err != nil {
		return nil, err
	}
	return &domain.Policy{
		PolicyNo: r.PolicyNo, ContractID: r.ContractID,
		SumInsured: centsToMoney(r.SumInsuredCents), OriginalPremium: centsToMoney(r.OriginalPremiumCents),
		RegionTag: r.RegionTag, StartDate: start, EndDate: end,
	}, nil
}

// lossDTO is the JSON shape for a loss.
type lossDTO struct {
	ID              int64  `json:"id"`
	ClaimNo         string `json:"claim_no"`
	PolicyID        int64  `json:"policy_id"`
	ContractID      int64  `json:"contract_id"`
	OccurrenceDate  string `json:"occurrence_date"`
	PaidAmountCents int64  `json:"paid_amount_cents"`
	EventTag        string `json:"event_tag"`
	Status          string `json:"status"`
}

func lossToDTO(l *domain.Loss) lossDTO {
	return lossDTO{
		ID: l.ID, ClaimNo: l.ClaimNo, PolicyID: l.PolicyID, ContractID: l.ContractID,
		OccurrenceDate: l.OccurrenceDate.Format("2006-01-02"),
		PaidAmountCents: moneyToCents(l.PaidAmount), EventTag: l.EventTag, Status: string(l.Status),
	}
}

type lossCreateReq struct {
	ClaimNo         string `json:"claim_no"`
	PolicyID        int64  `json:"policy_id"`
	OccurrenceDate  string `json:"occurrence_date"`
	PaidAmountCents int64  `json:"paid_amount_cents"`
	EventTag        string `json:"event_tag"`
}

func (r lossCreateReq) toDomain() (*domain.Loss, error) {
	occ, err := parseDate(r.OccurrenceDate)
	if err != nil {
		return nil, err
	}
	return &domain.Loss{
		ClaimNo: r.ClaimNo, PolicyID: r.PolicyID, OccurrenceDate: occ,
		PaidAmount: centsToMoney(r.PaidAmountCents), EventTag: r.EventTag,
	}, nil
}

// allocationDTO is the JSON shape for an allocation layer.
type allocationDTO struct {
	ID                  int64  `json:"id"`
	LossID              int64  `json:"loss_id"`
	ContractID          int64  `json:"contract_id"`
	ContractType        string `json:"contract_type"`
	Reinsurer           string `json:"reinsurer"`
	RecoveredCents      int64  `json:"recovered_cents"`
	AttachmentConsumedCents int64 `json:"attachment_consumed_cents"`
	LimitConsumedCents  int64  `json:"limit_consumed_cents"`
	Kind                string `json:"kind"`
}

func allocationToDTO(a *domain.Allocation) allocationDTO {
	return allocationDTO{
		ID: a.ID, LossID: a.LossID, ContractID: a.ContractID, ContractType: string(a.ContractType),
		Reinsurer: a.Reinsurer, RecoveredCents: moneyToCents(a.RecoveredAmount),
		AttachmentConsumedCents: moneyToCents(a.AttachmentConsumed), LimitConsumedCents: moneyToCents(a.LimitConsumed),
		Kind: string(a.Kind),
	}
}

// bordereauxDTO is the JSON shape for a quarterly account.
type bordereauxDTO struct {
	ID                      int64  `json:"id"`
	ContractID              int64  `json:"contract_id"`
	PeriodYear              int    `json:"period_year"`
	PeriodQuarter           int    `json:"period_quarter"`
	CededPremiumCents       int64  `json:"ceded_premium_cents"`
	RecoveredLossCents      int64  `json:"recovered_loss_cents"`
	CedingCommissionCents   int64  `json:"ceding_commission_cents"`
	BrokerFeeCents          int64  `json:"broker_fee_cents"`
	ReinstatementPremiumCents int64 `json:"reinstatement_premium_cents"`
	NetBalanceCents         int64  `json:"net_balance_cents"`
	Status                  string `json:"status"`
}

func bordereauxToDTO(b *domain.Bordereaux) bordereauxDTO {
	return bordereauxDTO{
		ID: b.ID, ContractID: b.ContractID, PeriodYear: b.PeriodYear, PeriodQuarter: b.PeriodQuarter,
		CededPremiumCents: moneyToCents(b.CededPremium), RecoveredLossCents: moneyToCents(b.RecoveredLoss),
		CedingCommissionCents: moneyToCents(b.CedingCommission), BrokerFeeCents: moneyToCents(b.BrokerFee),
		ReinstatementPremiumCents: moneyToCents(b.ReinstatementPremium), NetBalanceCents: moneyToCents(b.NetBalance),
		Status: string(b.Status),
	}
}
