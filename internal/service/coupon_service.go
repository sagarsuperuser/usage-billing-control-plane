package service

import (
	"fmt"
	"regexp"
	"strings"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type CouponService struct {
	store store.Repository
}

var (
	couponCodeInvalidRE = regexp.MustCompile(`[^a-z0-9_-]+`)
	couponCodeMultiRE   = regexp.MustCompile(`_+`)
)

func NewCouponService(s store.Repository) *CouponService {
	return &CouponService{store: s}
}

func (s *CouponService) CreateCoupon(input domain.Coupon) (domain.Coupon, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Code = normalizeCouponCode(input.Code)
	input.Status = normalizeCouponStatus(input.Status)
	input.DiscountType = normalizeCouponDiscountType(input.DiscountType)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))

	if err := validateCoupon(input); err != nil {
		return domain.Coupon{}, err
	}
	coupon, err := s.store.CreateCoupon(input)
	if err != nil {
		if err == store.ErrDuplicateKey {
			return domain.Coupon{}, fmt.Errorf("%w: coupon code already exists", ErrValidation)
		}
		return domain.Coupon{}, err
	}
	return coupon, nil
}

func (s *CouponService) ListCoupons(tenantID string) ([]domain.Coupon, error) {
	return s.store.ListCoupons(normalizeTenantID(tenantID))
}

func (s *CouponService) GetCoupon(tenantID, id string) (domain.Coupon, error) {
	return s.store.GetCoupon(normalizeTenantID(tenantID), strings.TrimSpace(id))
}

func normalizeCouponCode(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	raw = couponCodeInvalidRE.ReplaceAllString(raw, "_")
	raw = couponCodeMultiRE.ReplaceAllString(raw, "_")
	return strings.Trim(raw, "_")
}

func normalizeCouponStatus(raw domain.CouponStatus) domain.CouponStatus {
	switch domain.CouponStatus(strings.ToLower(strings.TrimSpace(string(raw)))) {
	case domain.CouponStatusActive:
		return domain.CouponStatusActive
	case domain.CouponStatusArchived:
		return domain.CouponStatusArchived
	default:
		return domain.CouponStatusDraft
	}
}

func normalizeCouponDiscountType(raw domain.CouponDiscountType) domain.CouponDiscountType {
	switch domain.CouponDiscountType(strings.ToLower(strings.TrimSpace(string(raw)))) {
	case domain.CouponDiscountTypePercentOff:
		return domain.CouponDiscountTypePercentOff
	default:
		return domain.CouponDiscountTypeAmountOff
	}
}

func validateCoupon(input domain.Coupon) error {
	if input.TenantID == "" {
		return fmt.Errorf("%w: tenant_id is required", ErrValidation)
	}
	if input.Code == "" {
		return fmt.Errorf("%w: code is required", ErrValidation)
	}
	if len(input.Code) > 64 {
		return fmt.Errorf("%w: code length must be <= 64", ErrValidation)
	}
	if input.Name == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if input.Status == "" {
		return fmt.Errorf("%w: status is required", ErrValidation)
	}
	if input.DiscountType == "" {
		return fmt.Errorf("%w: discount_type is required", ErrValidation)
	}
	switch input.DiscountType {
	case domain.CouponDiscountTypeAmountOff:
		if input.Currency == "" || len(input.Currency) != 3 {
			return fmt.Errorf("%w: currency must be a 3-letter code for amount_off coupons", ErrValidation)
		}
		if input.AmountOffCents <= 0 {
			return fmt.Errorf("%w: amount_off_cents must be > 0", ErrValidation)
		}
		if input.PercentOff != 0 {
			return fmt.Errorf("%w: percent_off must be 0 for amount_off coupons", ErrValidation)
		}
	case domain.CouponDiscountTypePercentOff:
		if input.PercentOff <= 0 || input.PercentOff > 100 {
			return fmt.Errorf("%w: percent_off must be between 1 and 100", ErrValidation)
		}
		if input.AmountOffCents != 0 {
			return fmt.Errorf("%w: amount_off_cents must be 0 for percent_off coupons", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: unsupported discount_type", ErrValidation)
	}
	return nil
}
