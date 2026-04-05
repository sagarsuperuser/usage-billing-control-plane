package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

// InvoiceGenerationService generates invoices from subscriptions.
// It aggregates usage, applies the pricing engine, adds base fees, add-ons,
// coupon discounts, and taxes. The result is an idempotent, draft invoice
// ready for finalization.
type InvoiceGenerationService struct {
	store     store.Repository
	numberGen *InvoiceNumberGenerator
}

func NewInvoiceGenerationService(repo store.Repository, db *sql.DB) *InvoiceGenerationService {
	return &InvoiceGenerationService{
		store:     repo,
		numberGen: NewInvoiceNumberGenerator(db),
	}
}

type GenerateInvoiceInput struct {
	TenantID       string
	SubscriptionID string
	PeriodStart    time.Time
	PeriodEnd      time.Time
}

type GenerateInvoiceResult struct {
	Invoice       domain.Invoice
	LineItems     []domain.InvoiceLineItem
	AlreadyExists bool
}

// Generate creates a draft invoice for a subscription's billing period.
// It is idempotent: if an invoice already exists for the (subscription, period)
// pair, it returns AlreadyExists=true without creating a duplicate.
func (s *InvoiceGenerationService) Generate(ctx context.Context, input GenerateInvoiceInput) (GenerateInvoiceResult, error) {
	// 1. Load subscription, customer, plan.
	subscription, err := s.store.GetSubscription(input.TenantID, input.SubscriptionID)
	if err != nil {
		return GenerateInvoiceResult{}, fmt.Errorf("load subscription: %w", err)
	}
	plan, err := s.store.GetPlan(input.TenantID, subscription.PlanID)
	if err != nil {
		return GenerateInvoiceResult{}, fmt.Errorf("load plan: %w", err)
	}

	// 2. Load workspace billing settings for tax codes, numbering, etc.
	settings, err := s.store.GetWorkspaceBillingSettings(input.TenantID)
	if err != nil {
		// Settings may not exist yet — use defaults.
		settings = domain.WorkspaceBillingSettings{}
	}

	// 3. Generate invoice number.
	prefix := settings.DocumentNumberPrefix
	number, err := s.numberGen.Next(ctx, input.TenantID, prefix)
	if err != nil {
		return GenerateInvoiceResult{}, fmt.Errorf("generate invoice number: %w", err)
	}

	// 4. Build line items.
	var lineItems []domain.InvoiceLineItem
	var subtotalCents int64

	// 4a. Base plan fee.
	if plan.BaseAmountCents > 0 {
		item := domain.InvoiceLineItem{
			TenantID:         input.TenantID,
			LineType:         domain.LineTypeBaseFee,
			Description:      fmt.Sprintf("Base fee — %s", plan.Name),
			Quantity:         1,
			UnitAmountCents:  plan.BaseAmountCents,
			AmountCents:      plan.BaseAmountCents,
			TotalAmountCents: plan.BaseAmountCents,
			BillingPeriodStart: &input.PeriodStart,
			BillingPeriodEnd:   &input.PeriodEnd,
		}
		lineItems = append(lineItems, item)
		subtotalCents += item.AmountCents
	}

	// 4b. Usage-based charges per meter.
	if len(plan.MeterIDs) > 0 {
		usageTotals, err := s.store.AggregateUsageForBillingPeriod(
			input.TenantID, input.SubscriptionID, plan.MeterIDs,
			input.PeriodStart, input.PeriodEnd,
		)
		if err != nil {
			return GenerateInvoiceResult{}, fmt.Errorf("aggregate usage: %w", err)
		}

		for _, meterID := range plan.MeterIDs {
			quantity := usageTotals[meterID]
			if quantity == 0 {
				continue
			}

			meter, err := s.store.GetMeter(input.TenantID, meterID)
			if err != nil {
				return GenerateInvoiceResult{}, fmt.Errorf("load meter %s: %w", meterID, err)
			}

			// Resolve the rating rule for this meter.
			rule, err := s.store.GetRatingRuleVersion(input.TenantID, meter.RatingRuleVersionID)
			if err != nil {
				return GenerateInvoiceResult{}, fmt.Errorf("load rating rule for meter %s: %w", meterID, err)
			}

			amountCents, err := domain.ComputeAmountCents(rule, quantity)
			if err != nil {
				return GenerateInvoiceResult{}, fmt.Errorf("compute amount for meter %s: %w", meterID, err)
			}

			item := domain.InvoiceLineItem{
				TenantID:            input.TenantID,
				LineType:            domain.LineTypeUsage,
				MeterID:             meterID,
				Description:         fmt.Sprintf("Usage — %s (%d %s)", meter.Name, quantity, meter.Unit),
				Quantity:            quantity,
				UnitAmountCents:     divSafe(amountCents, quantity),
				AmountCents:         amountCents,
				TotalAmountCents:    amountCents,
				PricingMode:         string(rule.Mode),
				RatingRuleVersionID: rule.ID,
				BillingPeriodStart:  &input.PeriodStart,
				BillingPeriodEnd:    &input.PeriodEnd,
			}
			lineItems = append(lineItems, item)
			subtotalCents += amountCents
		}
	}

	// 4c. Add-on charges.
	for _, addOnID := range plan.AddOnIDs {
		addOn, err := s.store.GetAddOn(input.TenantID, addOnID)
		if err != nil {
			return GenerateInvoiceResult{}, fmt.Errorf("load add-on %s: %w", addOnID, err)
		}
		item := domain.InvoiceLineItem{
			TenantID:         input.TenantID,
			LineType:         domain.LineTypeAddOn,
			AddOnID:          addOnID,
			Description:      fmt.Sprintf("Add-on — %s", addOn.Name),
			Quantity:         1,
			UnitAmountCents:  addOn.AmountCents,
			AmountCents:      addOn.AmountCents,
			TotalAmountCents: addOn.AmountCents,
			BillingPeriodStart: &input.PeriodStart,
			BillingPeriodEnd:   &input.PeriodEnd,
		}
		lineItems = append(lineItems, item)
		subtotalCents += addOn.AmountCents
	}

	// 4d. Coupon discounts.
	var discountCents int64
	for _, couponID := range plan.CouponIDs {
		coupon, err := s.store.GetCoupon(input.TenantID, couponID)
		if err != nil {
			return GenerateInvoiceResult{}, fmt.Errorf("load coupon %s: %w", couponID, err)
		}
		if coupon.Status != domain.CouponStatusActive {
			continue
		}

		var discountAmount int64
		switch coupon.DiscountType {
		case domain.CouponDiscountTypeAmountOff:
			discountAmount = coupon.AmountOffCents
		case domain.CouponDiscountTypePercentOff:
			discountAmount = int64(math.Round(float64(subtotalCents) * float64(coupon.PercentOff) / 100.0))
		}
		if discountAmount <= 0 {
			continue
		}

		item := domain.InvoiceLineItem{
			TenantID:         input.TenantID,
			LineType:         domain.LineTypeDiscount,
			CouponID:         couponID,
			Description:      fmt.Sprintf("Discount — %s", coupon.Name),
			Quantity:         1,
			AmountCents:      -discountAmount,
			TotalAmountCents: -discountAmount,
		}
		lineItems = append(lineItems, item)
		discountCents += discountAmount
	}

	// 4e. Tax computation.
	taxableAmount := subtotalCents - discountCents
	if taxableAmount < 0 {
		taxableAmount = 0
	}
	var totalTaxCents int64
	for _, taxCode := range settings.TaxCodes {
		tax, err := s.store.GetTaxByCode(input.TenantID, taxCode)
		if err != nil {
			continue // Skip taxes that can't be resolved.
		}
		if tax.Status != domain.TaxStatusActive {
			continue
		}
		taxAmount := int64(math.Round(float64(taxableAmount) * tax.Rate))

		item := domain.InvoiceLineItem{
			TenantID:         input.TenantID,
			LineType:         domain.LineTypeTax,
			TaxID:            tax.ID,
			Description:      fmt.Sprintf("Tax — %s", tax.Name),
			Quantity:         1,
			AmountCents:      taxAmount,
			TaxRate:          tax.Rate,
			TaxAmountCents:   taxAmount,
			TotalAmountCents: taxAmount,
		}
		lineItems = append(lineItems, item)
		totalTaxCents += taxAmount
	}

	// 5. Compute totals.
	totalAmountCents := subtotalCents - discountCents + totalTaxCents
	if totalAmountCents < 0 {
		totalAmountCents = 0
	}

	netPaymentTermDays := 0
	if settings.NetPaymentTermDays != nil {
		netPaymentTermDays = *settings.NetPaymentTermDays
	}

	dueAt := input.PeriodEnd.AddDate(0, 0, netPaymentTermDays)

	// 6. Create the invoice.
	invoice := domain.Invoice{
		TenantID:           input.TenantID,
		CustomerID:         subscription.CustomerID,
		SubscriptionID:     input.SubscriptionID,
		InvoiceNumber:      number,
		Status:             domain.InvoiceStatusDraft,
		PaymentStatus:      domain.InvoicePaymentPending,
		Currency:           plan.Currency,
		SubtotalCents:      subtotalCents,
		DiscountCents:      discountCents,
		TaxAmountCents:     totalTaxCents,
		TotalAmountCents:   totalAmountCents,
		AmountDueCents:     totalAmountCents,
		BillingPeriodStart: input.PeriodStart,
		BillingPeriodEnd:   input.PeriodEnd,
		DueAt:              &dueAt,
		NetPaymentTermDays: netPaymentTermDays,
		Memo:               settings.InvoiceMemo,
		Footer:             settings.InvoiceFooter,
	}

	created, err := s.store.CreateInvoice(invoice)
	if err != nil {
		if err == store.ErrAlreadyExists {
			return GenerateInvoiceResult{AlreadyExists: true}, nil
		}
		return GenerateInvoiceResult{}, fmt.Errorf("create invoice: %w", err)
	}

	// 7. Create line items.
	var createdItems []domain.InvoiceLineItem
	for _, item := range lineItems {
		item.InvoiceID = created.ID
		createdItem, err := s.store.CreateInvoiceLineItem(item)
		if err != nil {
			return GenerateInvoiceResult{}, fmt.Errorf("create line item: %w", err)
		}
		createdItems = append(createdItems, createdItem)
	}

	// 8. Advance the subscription's billing cycle.
	nextPeriodStart := input.PeriodEnd
	nextPeriodEnd := advanceBillingPeriod(nextPeriodStart, plan.BillingInterval)
	if err := s.store.UpdateSubscriptionBillingCycle(
		input.TenantID, input.SubscriptionID,
		input.PeriodStart, input.PeriodEnd, nextPeriodEnd,
	); err != nil {
		return GenerateInvoiceResult{}, fmt.Errorf("advance billing cycle: %w", err)
	}

	return GenerateInvoiceResult{
		Invoice:   created,
		LineItems: createdItems,
	}, nil
}

// Preview computes what the next invoice would look like for a subscription
// without persisting anything. Same calculation as Generate, no side effects.
func (s *InvoiceGenerationService) Preview(ctx context.Context, tenantID, subscriptionID string) (GenerateInvoiceResult, error) {
	subscription, err := s.store.GetSubscription(tenantID, subscriptionID)
	if err != nil {
		return GenerateInvoiceResult{}, fmt.Errorf("load subscription: %w", err)
	}
	plan, err := s.store.GetPlan(tenantID, subscription.PlanID)
	if err != nil {
		return GenerateInvoiceResult{}, fmt.Errorf("load plan: %w", err)
	}
	settings, err := s.store.GetWorkspaceBillingSettings(tenantID)
	if err != nil {
		settings = domain.WorkspaceBillingSettings{}
	}

	now := time.Now().UTC()
	periodStart := now.AddDate(0, -1, 0)
	if subscription.CurrentBillingPeriodStart != nil {
		periodStart = *subscription.CurrentBillingPeriodStart
	}
	periodEnd := now
	if subscription.CurrentBillingPeriodEnd != nil {
		periodEnd = *subscription.CurrentBillingPeriodEnd
	}

	var lineItems []domain.InvoiceLineItem
	var subtotalCents int64

	if plan.BaseAmountCents > 0 {
		item := domain.InvoiceLineItem{
			LineType:         domain.LineTypeBaseFee,
			Description:      fmt.Sprintf("Base fee — %s", plan.Name),
			Quantity:         1,
			UnitAmountCents:  plan.BaseAmountCents,
			AmountCents:      plan.BaseAmountCents,
			TotalAmountCents: plan.BaseAmountCents,
			BillingPeriodStart: &periodStart,
			BillingPeriodEnd:   &periodEnd,
		}
		lineItems = append(lineItems, item)
		subtotalCents += item.AmountCents
	}

	if len(plan.MeterIDs) > 0 {
		usageTotals, err := s.store.AggregateUsageForBillingPeriod(
			tenantID, subscriptionID, plan.MeterIDs, periodStart, periodEnd,
		)
		if err != nil {
			return GenerateInvoiceResult{}, fmt.Errorf("aggregate usage: %w", err)
		}
		for _, meterID := range plan.MeterIDs {
			quantity := usageTotals[meterID]
			if quantity == 0 {
				continue
			}
			meter, err := s.store.GetMeter(tenantID, meterID)
			if err != nil {
				continue
			}
			rule, err := s.store.GetRatingRuleVersion(tenantID, meter.RatingRuleVersionID)
			if err != nil {
				continue
			}
			amountCents, err := domain.ComputeAmountCents(rule, quantity)
			if err != nil {
				continue
			}
			lineItems = append(lineItems, domain.InvoiceLineItem{
				LineType:            domain.LineTypeUsage,
				MeterID:             meterID,
				Description:         fmt.Sprintf("Usage — %s (%d %s)", meter.Name, quantity, meter.Unit),
				Quantity:            quantity,
				UnitAmountCents:     divSafe(amountCents, quantity),
				AmountCents:         amountCents,
				TotalAmountCents:    amountCents,
				PricingMode:         string(rule.Mode),
				RatingRuleVersionID: rule.ID,
				BillingPeriodStart:  &periodStart,
				BillingPeriodEnd:    &periodEnd,
			})
			subtotalCents += amountCents
		}
	}

	for _, addOnID := range plan.AddOnIDs {
		addOn, err := s.store.GetAddOn(tenantID, addOnID)
		if err != nil {
			continue
		}
		lineItems = append(lineItems, domain.InvoiceLineItem{
			LineType:         domain.LineTypeAddOn,
			AddOnID:          addOnID,
			Description:      fmt.Sprintf("Add-on — %s", addOn.Name),
			Quantity:         1,
			UnitAmountCents:  addOn.AmountCents,
			AmountCents:      addOn.AmountCents,
			TotalAmountCents: addOn.AmountCents,
		})
		subtotalCents += addOn.AmountCents
	}

	var discountCents int64
	for _, couponID := range plan.CouponIDs {
		coupon, err := s.store.GetCoupon(tenantID, couponID)
		if err != nil || coupon.Status != domain.CouponStatusActive {
			continue
		}
		var discountAmount int64
		switch coupon.DiscountType {
		case domain.CouponDiscountTypeAmountOff:
			discountAmount = coupon.AmountOffCents
		case domain.CouponDiscountTypePercentOff:
			discountAmount = int64(math.Round(float64(subtotalCents) * float64(coupon.PercentOff) / 100.0))
		}
		if discountAmount <= 0 {
			continue
		}
		lineItems = append(lineItems, domain.InvoiceLineItem{
			LineType:         domain.LineTypeDiscount,
			CouponID:         couponID,
			Description:      fmt.Sprintf("Discount — %s", coupon.Name),
			Quantity:         1,
			AmountCents:      -discountAmount,
			TotalAmountCents: -discountAmount,
		})
		discountCents += discountAmount
	}

	taxableAmount := subtotalCents - discountCents
	if taxableAmount < 0 {
		taxableAmount = 0
	}
	var totalTaxCents int64
	for _, taxCode := range settings.TaxCodes {
		tax, err := s.store.GetTaxByCode(tenantID, taxCode)
		if err != nil || tax.Status != domain.TaxStatusActive {
			continue
		}
		taxAmount := int64(math.Round(float64(taxableAmount) * tax.Rate))
		lineItems = append(lineItems, domain.InvoiceLineItem{
			LineType:         domain.LineTypeTax,
			TaxID:            tax.ID,
			Description:      fmt.Sprintf("Tax — %s", tax.Name),
			Quantity:         1,
			AmountCents:      taxAmount,
			TaxRate:          tax.Rate,
			TaxAmountCents:   taxAmount,
			TotalAmountCents: taxAmount,
		})
		totalTaxCents += taxAmount
	}

	totalAmountCents := subtotalCents - discountCents + totalTaxCents
	if totalAmountCents < 0 {
		totalAmountCents = 0
	}

	netPaymentTermDays := 0
	if settings.NetPaymentTermDays != nil {
		netPaymentTermDays = *settings.NetPaymentTermDays
	}
	dueAt := periodEnd.AddDate(0, 0, netPaymentTermDays)

	invoice := domain.Invoice{
		TenantID:           tenantID,
		CustomerID:         subscription.CustomerID,
		SubscriptionID:     subscriptionID,
		InvoiceNumber:      "(preview)",
		Status:             domain.InvoiceStatusDraft,
		PaymentStatus:      domain.InvoicePaymentPending,
		Currency:           plan.Currency,
		SubtotalCents:      subtotalCents,
		DiscountCents:      discountCents,
		TaxAmountCents:     totalTaxCents,
		TotalAmountCents:   totalAmountCents,
		AmountDueCents:     totalAmountCents,
		BillingPeriodStart: periodStart,
		BillingPeriodEnd:   periodEnd,
		DueAt:              &dueAt,
		NetPaymentTermDays: netPaymentTermDays,
	}

	return GenerateInvoiceResult{Invoice: invoice, LineItems: lineItems}, nil
}

func advanceBillingPeriod(from time.Time, interval domain.BillingInterval) time.Time {
	switch interval {
	case domain.BillingIntervalYearly:
		return from.AddDate(1, 0, 0)
	default: // monthly
		return from.AddDate(0, 1, 0)
	}
}

func divSafe(total, divisor int64) int64 {
	if divisor == 0 {
		return 0
	}
	return total / divisor
}
