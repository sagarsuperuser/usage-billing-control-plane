package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"lago-usage-billing-alpha/internal/domain"
	"lago-usage-billing-alpha/internal/store"
)

type InvoiceService struct {
	store store.Repository
}

func NewInvoiceService(s store.Repository) *InvoiceService {
	return &InvoiceService{store: s}
}

func (s *InvoiceService) Preview(req domain.InvoicePreviewRequest) (domain.InvoicePreviewResponse, error) {
	req.TenantID = normalizeTenantID(req.TenantID)
	if strings.TrimSpace(req.CustomerID) == "" {
		return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: customer_id is required", ErrValidation)
	}
	if strings.TrimSpace(req.Currency) == "" {
		req.Currency = "USD"
	}
	if req.From != nil && req.To != nil && req.From.After(*req.To) {
		return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: from must be <= to", ErrValidation)
	}
	if len(req.Items) == 0 {
		var err error
		req.Items, err = s.itemsFromUsageEvents(req)
		if err != nil {
			return domain.InvoicePreviewResponse{}, err
		}
	}
	if len(req.Items) == 0 {
		return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: at least one item is required", ErrValidation)
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

		meter, err := s.store.GetMeter(req.TenantID, item.MeterID)
		if err != nil {
			return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: meter %s not found", ErrValidation, item.MeterID)
		}
		if normalizeTenantID(meter.TenantID) != req.TenantID {
			return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: meter %s not found", ErrValidation, item.MeterID)
		}

		ruleVersionID := meter.RatingRuleVersionID
		if strings.TrimSpace(item.RatingRuleVersionID) != "" {
			ruleVersionID = item.RatingRuleVersionID
		}
		rule, err := s.store.GetRatingRuleVersion(req.TenantID, ruleVersionID)
		if err != nil {
			return domain.InvoicePreviewResponse{}, fmt.Errorf("%w: rating rule %s not found", ErrValidation, ruleVersionID)
		}
		if normalizeTenantID(rule.TenantID) != req.TenantID {
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

func (s *InvoiceService) itemsFromUsageEvents(req domain.InvoicePreviewRequest) ([]domain.InvoicePreviewItem, error) {
	if req.From == nil || req.To == nil {
		return nil, fmt.Errorf("%w: from and to are required when items are omitted", ErrValidation)
	}

	events, err := s.store.ListUsageEvents(store.Filter{
		TenantID:   req.TenantID,
		CustomerID: strings.TrimSpace(req.CustomerID),
		From:       req.From,
		To:         req.To,
	})
	if err != nil {
		return nil, err
	}

	quantities := make(map[string]int64)
	for _, event := range events {
		quantities[event.MeterID] += event.Quantity
	}

	keys := make([]string, 0, len(quantities))
	for meterID := range quantities {
		keys = append(keys, meterID)
	}
	sort.Strings(keys)

	items := make([]domain.InvoicePreviewItem, 0, len(keys))
	for _, meterID := range keys {
		items = append(items, domain.InvoicePreviewItem{
			MeterID:  meterID,
			Quantity: quantities[meterID],
		})
	}
	return items, nil
}
