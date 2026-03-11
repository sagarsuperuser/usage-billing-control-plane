package domain

import (
	"errors"
)

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
