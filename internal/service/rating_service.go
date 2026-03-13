package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

var ErrValidation = errors.New("validation error")

type RatingService struct {
	store store.Repository
}

type ListRuleVersionsRequest struct {
	RuleKey        string `json:"rule_key,omitempty"`
	LifecycleState string `json:"lifecycle_state,omitempty"`
	LatestOnly     bool   `json:"latest_only,omitempty"`
}

var (
	ruleKeyInvalidRE = regexp.MustCompile(`[^a-z0-9_-]+`)
	ruleKeyMultiRE   = regexp.MustCompile(`_+`)
)

func NewRatingService(s store.Repository) *RatingService {
	return &RatingService{store: s}
}

func (s *RatingService) CreateRuleVersion(input domain.RatingRuleVersion) (domain.RatingRuleVersion, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Name = strings.TrimSpace(input.Name)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))

	rawRuleKey := strings.TrimSpace(input.RuleKey)
	if rawRuleKey == "" {
		rawRuleKey = input.Name
	}
	input.RuleKey = normalizeRuleKey(rawRuleKey)
	if input.RuleKey == "" {
		return domain.RatingRuleVersion{}, fmt.Errorf("%w: rule_key is required", ErrValidation)
	}
	if len(input.RuleKey) > 64 {
		return domain.RatingRuleVersion{}, fmt.Errorf("%w: rule_key length must be <= 64", ErrValidation)
	}

	lifecycleState, err := normalizeLifecycleState(input.LifecycleState)
	if err != nil {
		return domain.RatingRuleVersion{}, err
	}
	input.LifecycleState = lifecycleState

	if err := validateRatingRule(input); err != nil {
		return domain.RatingRuleVersion{}, err
	}
	return s.store.CreateRatingRuleVersion(input)
}

func (s *RatingService) ListRuleVersions(tenantID string, req ListRuleVersionsRequest) ([]domain.RatingRuleVersion, error) {
	tenantID = normalizeTenantID(tenantID)

	ruleKey := strings.TrimSpace(req.RuleKey)
	if ruleKey != "" {
		ruleKey = normalizeRuleKey(ruleKey)
		if ruleKey == "" {
			return nil, fmt.Errorf("%w: rule_key is invalid", ErrValidation)
		}
	}

	lifecycleState, err := normalizeLifecycleStateFilter(req.LifecycleState)
	if err != nil {
		return nil, err
	}

	return s.store.ListRatingRuleVersions(store.RatingRuleListFilter{
		TenantID:       tenantID,
		RuleKey:        ruleKey,
		LifecycleState: string(lifecycleState),
		LatestOnly:     req.LatestOnly,
	})
}

func (s *RatingService) GetRuleVersion(tenantID, id string) (domain.RatingRuleVersion, error) {
	return s.store.GetRatingRuleVersion(normalizeTenantID(tenantID), id)
}

func validateRatingRule(input domain.RatingRuleVersion) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if strings.TrimSpace(input.RuleKey) == "" {
		return fmt.Errorf("%w: rule_key is required", ErrValidation)
	}
	if input.LifecycleState == "" {
		return fmt.Errorf("%w: lifecycle_state is required", ErrValidation)
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

func normalizeRuleKey(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	raw = ruleKeyInvalidRE.ReplaceAllString(raw, "_")
	raw = ruleKeyMultiRE.ReplaceAllString(raw, "_")
	raw = strings.Trim(raw, "_")
	return raw
}

func normalizeLifecycleState(raw domain.RatingRuleLifecycleState) (domain.RatingRuleLifecycleState, error) {
	state := domain.RatingRuleLifecycleState(strings.ToLower(strings.TrimSpace(string(raw))))
	if state == "" {
		return domain.RatingRuleLifecycleActive, nil
	}
	switch state {
	case domain.RatingRuleLifecycleDraft, domain.RatingRuleLifecycleActive, domain.RatingRuleLifecycleArchived:
		return state, nil
	default:
		return "", fmt.Errorf("%w: lifecycle_state must be draft, active, or archived", ErrValidation)
	}
}

func normalizeLifecycleStateFilter(raw string) (domain.RatingRuleLifecycleState, error) {
	state := domain.RatingRuleLifecycleState(strings.ToLower(strings.TrimSpace(raw)))
	if state == "" {
		return "", nil
	}
	switch state {
	case domain.RatingRuleLifecycleDraft, domain.RatingRuleLifecycleActive, domain.RatingRuleLifecycleArchived:
		return state, nil
	default:
		return "", fmt.Errorf("%w: lifecycle_state must be draft, active, or archived", ErrValidation)
	}
}
