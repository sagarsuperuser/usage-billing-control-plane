package service

import (
	"fmt"
	"strings"

	"lago-usage-billing-alpha/internal/domain"
	"lago-usage-billing-alpha/internal/store"
)

type UsageService struct {
	store *store.MemoryStore
}

func NewUsageService(s *store.MemoryStore) *UsageService {
	return &UsageService{store: s}
}

func (s *UsageService) CreateUsageEvent(input domain.UsageEvent) (domain.UsageEvent, error) {
	if strings.TrimSpace(input.CustomerID) == "" || strings.TrimSpace(input.MeterID) == "" {
		return domain.UsageEvent{}, fmt.Errorf("%w: customer_id and meter_id are required", ErrValidation)
	}
	if input.Quantity < 0 {
		return domain.UsageEvent{}, fmt.Errorf("%w: quantity must be >= 0", ErrValidation)
	}
	if _, err := s.store.GetMeter(input.MeterID); err != nil {
		return domain.UsageEvent{}, fmt.Errorf("%w: meter not found", ErrValidation)
	}
	return s.store.CreateUsageEvent(input)
}

func (s *UsageService) CreateBilledEntry(input domain.BilledEntry) (domain.BilledEntry, error) {
	if strings.TrimSpace(input.CustomerID) == "" || strings.TrimSpace(input.MeterID) == "" {
		return domain.BilledEntry{}, fmt.Errorf("%w: customer_id and meter_id are required", ErrValidation)
	}
	if _, err := s.store.GetMeter(input.MeterID); err != nil {
		return domain.BilledEntry{}, fmt.Errorf("%w: meter not found", ErrValidation)
	}
	return s.store.CreateBilledEntry(input)
}
