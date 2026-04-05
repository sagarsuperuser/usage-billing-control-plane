package domain

import (
	"errors"
	"time"
)

type PricingMode string

const (
	PricingModeFlat      PricingMode = "flat"
	PricingModeGraduated PricingMode = "graduated"
	PricingModePackage   PricingMode = "package"
)

type RatingTier struct {
	UpTo            int64 `json:"up_to"`
	UnitAmountCents int64 `json:"unit_amount_cents"`
}

type RatingRuleLifecycleState string

const (
	RatingRuleLifecycleDraft    RatingRuleLifecycleState = "draft"
	RatingRuleLifecycleActive   RatingRuleLifecycleState = "active"
	RatingRuleLifecycleArchived RatingRuleLifecycleState = "archived"
)

type RatingRuleVersion struct {
	ID                     string                   `json:"id"`
	TenantID               string                   `json:"tenant_id,omitempty"`
	RuleKey                string                   `json:"rule_key"`
	Name                   string                   `json:"name"`
	Version                int                      `json:"version"`
	LifecycleState         RatingRuleLifecycleState `json:"lifecycle_state,omitempty"`
	Mode                   PricingMode              `json:"mode"`
	Currency               string                   `json:"currency"`
	FlatAmountCents        int64                    `json:"flat_amount_cents"`
	GraduatedTiers         []RatingTier             `json:"graduated_tiers"`
	PackageSize            int64                    `json:"package_size"`
	PackageAmountCents     int64                    `json:"package_amount_cents"`
	OverageUnitAmountCents int64                    `json:"overage_unit_amount_cents"`
	CreatedAt              time.Time                `json:"created_at"`
}

type Meter struct {
	ID                  string    `json:"id"`
	TenantID            string    `json:"tenant_id,omitempty"`
	Key                 string    `json:"key"`
	Name                string    `json:"name"`
	Unit                string    `json:"unit"`
	Aggregation         string    `json:"aggregation"`
	RatingRuleVersionID string    `json:"rating_rule_version_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type BillingInterval string

const (
	BillingIntervalMonthly BillingInterval = "monthly"
	BillingIntervalYearly  BillingInterval = "yearly"
)

type PlanStatus string

const (
	PlanStatusDraft    PlanStatus = "draft"
	PlanStatusActive   PlanStatus = "active"
	PlanStatusArchived PlanStatus = "archived"
)

type AddOnStatus string

const (
	AddOnStatusDraft    AddOnStatus = "draft"
	AddOnStatusActive   AddOnStatus = "active"
	AddOnStatusArchived AddOnStatus = "archived"
)

type AddOn struct {
	ID              string          `json:"id"`
	TenantID        string          `json:"tenant_id,omitempty"`
	Code            string          `json:"code"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Currency        string          `json:"currency"`
	BillingInterval BillingInterval `json:"billing_interval"`
	Status          AddOnStatus     `json:"status"`
	AmountCents     int64           `json:"amount_cents"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type CouponStatus string

const (
	CouponStatusDraft    CouponStatus = "draft"
	CouponStatusActive   CouponStatus = "active"
	CouponStatusArchived CouponStatus = "archived"
)

type CouponDiscountType string

const (
	CouponDiscountTypeAmountOff  CouponDiscountType = "amount_off"
	CouponDiscountTypePercentOff CouponDiscountType = "percent_off"
)

type CouponFrequency string

const (
	CouponFrequencyOnce      CouponFrequency = "once"
	CouponFrequencyRecurring CouponFrequency = "recurring"
	CouponFrequencyForever   CouponFrequency = "forever"
)

type Coupon struct {
	ID                string             `json:"id"`
	TenantID          string             `json:"tenant_id,omitempty"`
	Code              string             `json:"code"`
	Name              string             `json:"name"`
	Description       string             `json:"description,omitempty"`
	Status            CouponStatus       `json:"status"`
	DiscountType      CouponDiscountType `json:"discount_type"`
	Currency          string             `json:"currency,omitempty"`
	AmountOffCents    int64              `json:"amount_off_cents"`
	PercentOff        int                `json:"percent_off"`
	Frequency         CouponFrequency    `json:"frequency"`
	FrequencyDuration int                `json:"frequency_duration"`
	ExpirationAt      *time.Time         `json:"expiration_at,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

type TaxStatus string

const (
	TaxStatusDraft    TaxStatus = "draft"
	TaxStatusActive   TaxStatus = "active"
	TaxStatusArchived TaxStatus = "archived"
)

type Tax struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id,omitempty"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      TaxStatus `json:"status"`
	Rate        float64   `json:"rate"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Plan struct {
	ID              string          `json:"id"`
	TenantID        string          `json:"tenant_id,omitempty"`
	Code            string          `json:"code"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Currency        string          `json:"currency"`
	BillingInterval BillingInterval `json:"billing_interval"`
	Status          PlanStatus      `json:"status"`
	BaseAmountCents int64           `json:"base_amount_cents"`
	MeterIDs        []string        `json:"meter_ids"`
	AddOnIDs        []string        `json:"add_on_ids"`
	CouponIDs       []string        `json:"coupon_ids"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

var ErrInvalidPricingConfig = errors.New("invalid pricing config")

func ComputeAmountCents(rule RatingRuleVersion, quantity int64) (int64, error) {
	if quantity < 0 {
		return 0, ErrInvalidPricingConfig
	}

	switch rule.Mode {
	case PricingModeFlat:
		if rule.FlatAmountCents < 0 {
			return 0, ErrInvalidPricingConfig
		}
		if quantity == 0 {
			return 0, nil
		}
		return rule.FlatAmountCents, nil
	case PricingModeGraduated:
		if len(rule.GraduatedTiers) == 0 {
			return 0, ErrInvalidPricingConfig
		}
		remaining := quantity
		lastUpper := int64(0)
		amount := int64(0)
		for i, tier := range rule.GraduatedTiers {
			if tier.UnitAmountCents < 0 {
				return 0, ErrInvalidPricingConfig
			}
			if tier.UpTo < 0 {
				return 0, ErrInvalidPricingConfig
			}

			if remaining == 0 {
				break
			}

			if tier.UpTo == 0 {
				amount += remaining * tier.UnitAmountCents
				remaining = 0
				break
			}

			if tier.UpTo < lastUpper {
				return 0, ErrInvalidPricingConfig
			}

			tierCapacity := tier.UpTo - lastUpper
			if i == 0 {
				tierCapacity = tier.UpTo
			}
			if tierCapacity < 0 {
				return 0, ErrInvalidPricingConfig
			}

			consumed := minInt64(remaining, tierCapacity)
			amount += consumed * tier.UnitAmountCents
			remaining -= consumed
			lastUpper = tier.UpTo
		}

		if remaining > 0 {
			return 0, ErrInvalidPricingConfig
		}

		return amount, nil
	case PricingModePackage:
		if rule.PackageSize <= 0 || rule.PackageAmountCents < 0 || rule.OverageUnitAmountCents < 0 {
			return 0, ErrInvalidPricingConfig
		}
		if quantity == 0 {
			return 0, nil
		}

		fullPackages := quantity / rule.PackageSize
		remainder := quantity % rule.PackageSize
		amount := fullPackages * rule.PackageAmountCents
		amount += remainder * rule.OverageUnitAmountCents
		return amount, nil
	default:
		return 0, ErrInvalidPricingConfig
	}
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
