package service

import (
	"errors"
	"fmt"
	"strings"

	"lago-usage-billing-alpha/internal/domain"
	"lago-usage-billing-alpha/internal/store"
)

var ErrValidation = errors.New("validation error")

type RatingService struct {
	store *store.MemoryStore
}

func NewRatingService(s *store.MemoryStore) *RatingService {
	return &RatingService{store: s}
}

func (s *RatingService) CreateRuleVersion(input domain.RatingRuleVersion) (domain.RatingRuleVersion, error) {
	if err := validateRatingRule(input); err != nil {
		return domain.RatingRuleVersion{}, err
	}
	return s.store.CreateRatingRuleVersion(input)
}

func (s *RatingService) ListRuleVersions() []domain.RatingRuleVersion {
	return s.store.ListRatingRuleVersions()
}

func (s *RatingService) GetRuleVersion(id string) (domain.RatingRuleVersion, error) {
	return s.store.GetRatingRuleVersion(id)
}

func validateRatingRule(input domain.RatingRuleVersion) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if input.Version <= 0 {
		return fmt.Errorf("%w: version must be > 0", ErrValidation)
	}
	if strings.TrimSpace(input.Currency) == "" {
		return fmt.Errorf("%w: currency is required", ErrValidation)
	}

	switch input.Mode {
	case domain.PricingModeFlat:
		if input.FlatAmountCents < 0 {
			return fmt.Errorf("%w: flat_amount_cents must be >= 0", ErrValidation)
		}
	case domain.PricingModeGraduated:
		if len(input.GraduatedTiers) == 0 {
			return fmt.Errorf("%w: graduated_tiers required", ErrValidation)
		}
		for i, t := range input.GraduatedTiers {
			if t.UnitAmountCents < 0 {
				return fmt.Errorf("%w: tier %d unit amount must be >= 0", ErrValidation, i)
			}
			if t.UpTo < 0 {
				return fmt.Errorf("%w: tier %d up_to must be >= 0", ErrValidation, i)
			}
		}
	case domain.PricingModePackage:
		if input.PackageSize <= 0 {
			return fmt.Errorf("%w: package_size must be > 0", ErrValidation)
		}
		if input.PackageAmountCents < 0 || input.OverageUnitAmountCents < 0 {
			return fmt.Errorf("%w: package amount and overage must be >= 0", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: unsupported pricing mode", ErrValidation)
	}

	if _, err := domain.ComputeAmountCents(input, 10); err != nil {
		return fmt.Errorf("%w: %s", ErrValidation, err)
	}

	return nil
}
