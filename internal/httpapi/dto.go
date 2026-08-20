package httpapi

import (
	"strconv"
	"time"

	"task126-reinsurance/internal/domain"
)

// contractDTO is the JSON shape for a contract. Money fields use *_cents.
type contractDTO struct {
	ID                   int64  `json:"id"`
	Code                 string `json:"code"`
	Type                 string `json:"type"`
	AttachmentCents      int64  `json:"attachment_cents"`
	LimitCents           int64  `json:"limit_cents"`
	CessionRate          float64 `json:"cession_rate"`
	RetainedLineCents    int64  `json:"retained_line_cents"`
	TreatyCapacity       int    `json:"treaty_capacity"`
	NumReinstatements    int    `json:"num_reinstatements"`
	ReinstatementFactor  float64 `json:"reinstatement_factor"`
	CedingCommissionRate float64 `json:"ceding_commission_rate"`
	BrokerRate           float64 `json:"broker_rate"`
	CededPremiumRate     float64 `json:"ceded_premium_rate"`
	Currency             string `json:"currency"`
	StartDate            string `json:"start_date"`
	EndDate              string `json:"end_date"`
	Status               string `json:"status"`
}

func contractToDTO(c *domain.Contract) contractDTO {
	return contractDTO{
		ID: c.ID, Code: c.Code, Type: string(c.Type),
		AttachmentCents: moneyToCents(c.AttachmentPoint), LimitCents: moneyToCents(c.Limit),
		CessionRate: c.CessionRate, RetainedLineCents: moneyToCents(c.RetainedLine),
		TreatyCapacity: c.TreatyCapacity, NumReinstatements: c.NumReinstatements,
		ReinstatementFactor: c.ReinstatementFactor, CedingCommissionRate: c.CedingCommissionRate,
		BrokerRate: c.BrokerRate, CededPremiumRate: c.CededPremiumRate, Currency: c.Currency,
		StartDate: c.StartDate.Format("2006-01-02"), EndDate: c.EndDate.Format("2006-01-02"),
		Status: string(c.Status),
	}
}

type contractCreateReq struct {
	Code                 string  `json:"code"`
	Type                 string  `json:"type"`
	AttachmentCents     int64   `json:"attachment_cents"`
	LimitCents           int64   `json:"limit_cents"`
	CessionRate          float64 `json:"cession_rate"`
	RetainedLineCents    int64   `json:"retained_line_cents"`
	TreatyCapacity       int     `json:"treaty_capacity"`
	NumReinstatements    int     `json:"num_reinstatements"`
	ReinstatementFactor  float64 `json:"reinstatement_factor"`
	CedingCommissionRate float64 `json:"ceding_commission_rate"`
	BrokerRate           float64 `json:"broker_rate"`
	CededPremiumRate     float64 `json:"ceded_premium_rate"`
	Currency             string  `json:"currency"`
	StartDate            string  `json:"start_date"`
	EndDate              string  `json:"end_date"`
}

func (r contractCreateReq) toDomain() (*domain.Contract, error) {
	start, err := parseDate(r.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := parseDate(r.EndDate)
	if err != nil {
		return nil, err
	}
	return &domain.Contract{
		Code: r.Code, Type: domain.ContractType(r.Type),
		AttachmentPoint: centsToMoney(r.AttachmentCents), Limit: centsToMoney(r.LimitCents),
		CessionRate: r.CessionRate, RetainedLine: centsToMoney(r.RetainedLineCents),
		TreatyCapacity: r.TreatyCapacity, NumReinstatements: r.NumReinstatements,
		ReinstatementFactor: r.ReinstatementFactor, CedingCommissionRate: r.CedingCommissionRate,
		BrokerRate: r.BrokerRate, CededPremiumRate: r.CededPremiumRate, Currency: r.Currency,
		StartDate: start, EndDate: end,
	}, nil
}

type statusUpdateReq struct {
	Status string `json:"status"`
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, domain.Newf(domain.ErrInvalidArgument, "date is required")
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, domain.Newf(domain.ErrInvalidArgument, "invalid date %q", s)
	}
	return t, nil
}

func parsePathID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.Newf(domain.ErrInvalidArgument, "invalid id %q", s)
	}
	return id, nil
}
