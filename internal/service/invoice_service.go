package service

import (
	"fmt"
	"strings"
	"time"

	"lago-usage-billing-alpha/internal/domain"
	"lago-usage-billing-alpha/internal/store"
)

type InvoiceService struct {
	store *store.MemoryStore
}

func NewInvoiceService(s *store.MemoryStore) *InvoiceService {
	return &InvoiceService{store: s}
}

func (s *InvoiceService) Preview(req domain.InvoicePreviewRequest) (domain.InvoicePreviewResponse, error) {
	if strings.TrimSpace(req.CustomerID) == "" {
		return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: customer_id is required", ErrValidation)
	}
	if len(req.Items) == 0 {
		return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: at least one item is required", ErrValidation)
	}
	if strings.TrimSpace(req.Currency) == "" {
		req.Currency = "USD"
	}

	resp := domain.InvoicePreviewResponse{
		CustomerID:  req.CustomerID,
		Currency:    req.Currency,
		Lines:       make([]domain.InvoicePreviewLine, 0, len(req.Items)),
		GeneratedAt: time.Now().UTC(),
	}

	for _, item := range req.Items {
		if item.Quantity < 0 {
			return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: quantity must be >= 0", ErrValidation)
		}

		meter, err := s.store.GetMeter(item.MeterID)
		if err != nil {
			return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: meter %s not found", ErrValidation, item.MeterID)
		}

		ruleVersionID := meter.RatingRuleVersionID
		if strings.TrimSpace(item.RatingRuleVersionID) != "" {
			ruleVersionID = item.RatingRuleVersionID
		}
		rule, err := s.store.GetRatingRuleVersion(ruleVersionID)
		if err != nil {
			return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: rating rule %s not found", ErrValidation, ruleVersionID)
		}

		amount, err := domain.ComputeAmountCents(rule, item.Quantity)
		if err != nil {
			return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: unable to compute amount for meter %s", ErrValidation, item.MeterID)
		}

		line := domain.InvoicePreviewLine{
			MeterID:       item.MeterID,
			Quantity:      item.Quantity,
			Mode:          rule.Mode,
			AmountCents:   amount,
			RuleVersionID: rule.ID,
		}
		resp.Lines = append(resp.Lines, line)
		resp.TotalCents += amount
	}

	return resp, nil
}
