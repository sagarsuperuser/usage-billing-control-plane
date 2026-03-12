package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"lago-usage-billing-alpha/internal/domain"
)

const (
	DefaultInvoiceExplainabilitySort = "created_at_asc"
)

var allowedInvoiceExplainabilitySorts = map[string]struct{}{
	"amount_cents_asc":  {},
	"amount_cents_desc": {},
	"created_at_asc":    {},
	"created_at_desc":   {},
}

type InvoiceExplainabilityOptions struct {
	FeeTypes     map[string]struct{}
	LineItemSort string
	Page         int
	Limit        int
}

func NewInvoiceExplainabilityOptions(feeTypes []string, lineItemSort string, page, limit int) (InvoiceExplainabilityOptions, error) {
	out := InvoiceExplainabilityOptions{
		FeeTypes:     make(map[string]struct{}),
		LineItemSort: strings.TrimSpace(strings.ToLower(lineItemSort)),
		Page:         page,
		Limit:        limit,
	}

	if out.LineItemSort == "" {
		out.LineItemSort = DefaultInvoiceExplainabilitySort
	}
	if _, ok := allowedInvoiceExplainabilitySorts[out.LineItemSort]; !ok {
		return InvoiceExplainabilityOptions{}, fmt.Errorf("%w: line_item_sort must be one of amount_cents_asc, amount_cents_desc, created_at_asc, created_at_desc", ErrValidation)
	}
	if out.Page < 0 {
		return InvoiceExplainabilityOptions{}, fmt.Errorf("%w: page must be >= 0", ErrValidation)
	}
	if out.Limit < 0 {
		return InvoiceExplainabilityOptions{}, fmt.Errorf("%w: limit must be >= 0", ErrValidation)
	}

	for _, raw := range feeTypes {
		v := strings.TrimSpace(strings.ToLower(raw))
		if v == "" {
			continue
		}
		out.FeeTypes[v] = struct{}{}
	}

	return out, nil
}

type lagoInvoiceEnvelope struct {
	Invoice lagoInvoice `json:"invoice"`
}

type lagoInvoice struct {
	ID               string       `json:"lago_id"`
	Number           string       `json:"number"`
	Status           string       `json:"status"`
	Currency         string       `json:"currency"`
	TotalAmountCents int64        `json:"total_amount_cents"`
	Fees             []lagoFeeRaw `json:"fees"`
}

type lagoFeeRaw struct {
	ID                  string         `json:"lago_id"`
	LagoChargeID        string         `json:"lago_charge_id"`
	LagoSubscriptionID  string         `json:"lago_subscription_id"`
	AmountCents         int64          `json:"amount_cents"`
	TaxesAmountCents    int64          `json:"taxes_amount_cents"`
	TotalAmountCents    int64          `json:"total_amount_cents"`
	Units               *float64       `json:"units"`
	EventsCount         *int64         `json:"events_count"`
	CreatedAt           string         `json:"created_at"`
	FromDatetime        string         `json:"from_datetime"`
	ToDatetime          string         `json:"to_datetime"`
	ChargesFromDatetime string         `json:"charges_from_datetime"`
	ChargesToDatetime   string         `json:"charges_to_datetime"`
	AmountDetails       map[string]any `json:"amount_details"`
	Item                lagoFeeItemRaw `json:"item"`
}

type lagoFeeItemRaw struct {
	Type                     string `json:"type"`
	Code                     string `json:"code"`
	Name                     string `json:"name"`
	InvoiceDisplayName       string `json:"invoice_display_name"`
	FilterInvoiceDisplayName string `json:"filter_invoice_display_name"`
}

type explainabilityRow struct {
	Line      domain.InvoiceExplainabilityLineItem
	CreatedAt time.Time
}

func BuildInvoiceExplainabilityFromLago(payload []byte, options InvoiceExplainabilityOptions) (domain.InvoiceExplainability, error) {
	if !json.Valid(payload) {
		return domain.InvoiceExplainability{}, fmt.Errorf("%w: lago invoice payload must be valid json", ErrValidation)
	}

	var env lagoInvoiceEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return domain.InvoiceExplainability{}, fmt.Errorf("decode lago invoice payload: %w", err)
	}
	if strings.TrimSpace(env.Invoice.ID) == "" {
		return domain.InvoiceExplainability{}, fmt.Errorf("%w: lago invoice payload missing invoice", ErrValidation)
	}

	rows := make([]explainabilityRow, 0, len(env.Invoice.Fees))
	for _, fee := range env.Invoice.Fees {
		line := buildExplainabilityLineItem(fee)
		if !matchesFeeType(options, line.FeeType) {
			continue
		}
		rows = append(rows, explainabilityRow{Line: line, CreatedAt: parseTimeLoose(fee.CreatedAt)})
	}

	sortExplainabilityRows(rows, options.LineItemSort)
	allLines := make([]domain.InvoiceExplainabilityLineItem, 0, len(rows))
	for _, row := range rows {
		allLines = append(allLines, row.Line)
	}

	paged := paginateExplainabilityLines(allLines, options.Page, options.Limit)
	out := domain.InvoiceExplainability{
		InvoiceID:             strings.TrimSpace(env.Invoice.ID),
		InvoiceNumber:         strings.TrimSpace(env.Invoice.Number),
		InvoiceStatus:         strings.TrimSpace(env.Invoice.Status),
		Currency:              strings.TrimSpace(env.Invoice.Currency),
		GeneratedAt:           time.Now().UTC(),
		TotalAmountCents:      env.Invoice.TotalAmountCents,
		ExplainabilityVersion: "v1",
		LineItemsCount:        len(allLines),
		LineItems:             paged,
	}
	out.ExplainabilityDigest = buildExplainabilityDigest(out, allLines)
	return out, nil
}

func buildExplainabilityLineItem(fee lagoFeeRaw) domain.InvoiceExplainabilityLineItem {
	itemName := strings.TrimSpace(fee.Item.InvoiceDisplayName)
	if itemName == "" {
		itemName = strings.TrimSpace(fee.Item.Name)
	}
	if itemName == "" {
		itemName = strings.TrimSpace(fee.Item.Code)
	}
	if itemName == "" {
		itemName = strings.TrimSpace(fee.ID)
	}

	chargeModel := firstNonEmptyString(
		stringFromAny(fee.AmountDetails["charge_model"]),
		stringFromAny(fee.AmountDetails["model"]),
	)

	itemType := strings.TrimSpace(strings.ToLower(fee.Item.Type))
	computationMode := itemType
	if itemType == "charge" {
		if chargeModel != "" {
			computationMode = "charge:" + chargeModel
		} else {
			computationMode = "charge:unknown"
		}
	}
	if computationMode == "" {
		computationMode = "unknown"
	}

	ruleRef := buildRuleReference(itemType, strings.TrimSpace(fee.Item.Code), strings.TrimSpace(fee.LagoChargeID), strings.TrimSpace(fee.LagoSubscriptionID), strings.TrimSpace(fee.ID))
	from := firstTimePointer(fee.ChargesFromDatetime, fee.FromDatetime)
	to := firstTimePointer(fee.ChargesToDatetime, fee.ToDatetime)
	properties := normalizeJSONMap(fee.AmountDetails)
	if properties == nil {
		properties = map[string]any{}
	}
	billableMetricCode := firstNonEmptyString(
		stringFromAny(fee.AmountDetails["billable_metric_code"]),
		stringFromAny(fee.AmountDetails["metric_code"]),
	)

	totalAmount := fee.TotalAmountCents
	if totalAmount == 0 && (fee.AmountCents != 0 || fee.TaxesAmountCents != 0) {
		totalAmount = fee.AmountCents + fee.TaxesAmountCents
	}

	return domain.InvoiceExplainabilityLineItem{
		FeeID:                   strings.TrimSpace(fee.ID),
		FeeType:                 itemType,
		ItemName:                itemName,
		ItemCode:                strings.TrimSpace(fee.Item.Code),
		AmountCents:             fee.AmountCents,
		TaxesAmountCents:        fee.TaxesAmountCents,
		TotalAmountCents:        totalAmount,
		Units:                   fee.Units,
		EventsCount:             fee.EventsCount,
		ComputationMode:         computationMode,
		ChargeModel:             chargeModel,
		RuleReference:           ruleRef,
		FromDatetime:            from,
		ToDatetime:              to,
		ChargeFilterDisplayName: strings.TrimSpace(fee.Item.FilterInvoiceDisplayName),
		SubscriptionID:          strings.TrimSpace(fee.LagoSubscriptionID),
		ChargeID:                strings.TrimSpace(fee.LagoChargeID),
		BillableMetricCode:      strings.TrimSpace(billableMetricCode),
		Properties:              properties,
	}
}

func buildRuleReference(itemType, itemCode, chargeID, subscriptionID, fallback string) string {
	switch itemType {
	case "charge":
		if itemCode != "" {
			return "charge:" + itemCode
		}
		if chargeID != "" {
			return "charge:" + chargeID
		}
		return "charge:" + fallback
	case "subscription":
		if subscriptionID != "" {
			return "subscription:" + subscriptionID
		}
		return "subscription:" + fallback
	case "fixed_charge":
		return "fixed_charge:" + fallback
	case "add_on":
		if itemCode != "" {
			return "add_on:" + itemCode
		}
		return "add_on:" + fallback
	case "credit":
		return "credit:" + fallback
	default:
		if itemCode != "" {
			return itemType + ":" + itemCode
		}
		if strings.TrimSpace(itemType) == "" {
			return "unknown:" + fallback
		}
		return itemType + ":" + fallback
	}
}

func sortExplainabilityRows(rows []explainabilityRow, sortBy string) {
	sort.SliceStable(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]

		switch sortBy {
		case "amount_cents_asc":
			if left.Line.AmountCents != right.Line.AmountCents {
				return left.Line.AmountCents < right.Line.AmountCents
			}
		case "amount_cents_desc":
			if left.Line.AmountCents != right.Line.AmountCents {
				return left.Line.AmountCents > right.Line.AmountCents
			}
		case "created_at_desc":
			if !left.CreatedAt.Equal(right.CreatedAt) {
				return left.CreatedAt.After(right.CreatedAt)
			}
		default:
			if !left.CreatedAt.Equal(right.CreatedAt) {
				return left.CreatedAt.Before(right.CreatedAt)
			}
		}
		return left.Line.FeeID < right.Line.FeeID
	})
}

func paginateExplainabilityLines(lines []domain.InvoiceExplainabilityLineItem, page, limit int) []domain.InvoiceExplainabilityLineItem {
	if page == 0 && limit == 0 {
		return lines
	}
	if len(lines) == 0 {
		return []domain.InvoiceExplainabilityLineItem{}
	}

	pageValue := page
	if pageValue <= 0 {
		pageValue = 1
	}
	limitValue := limit
	if limitValue <= 0 {
		limitValue = len(lines)
	}
	offset := (pageValue - 1) * limitValue
	if offset >= len(lines) {
		return []domain.InvoiceExplainabilityLineItem{}
	}
	end := offset + limitValue
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end]
}

func matchesFeeType(options InvoiceExplainabilityOptions, feeType string) bool {
	if len(options.FeeTypes) == 0 {
		return true
	}
	_, ok := options.FeeTypes[strings.ToLower(strings.TrimSpace(feeType))]
	return ok
}

func buildExplainabilityDigest(head domain.InvoiceExplainability, lines []domain.InvoiceExplainabilityLineItem) string {
	type digestPayload struct {
		InvoiceID             string                                 `json:"invoice_id"`
		InvoiceNumber         string                                 `json:"invoice_number"`
		InvoiceStatus         string                                 `json:"invoice_status"`
		Currency              string                                 `json:"currency"`
		TotalAmountCents      int64                                  `json:"total_amount_cents"`
		ExplainabilityVersion string                                 `json:"explainability_version"`
		LineItems             []domain.InvoiceExplainabilityLineItem `json:"line_items"`
	}

	payload := digestPayload{
		InvoiceID:             head.InvoiceID,
		InvoiceNumber:         head.InvoiceNumber,
		InvoiceStatus:         head.InvoiceStatus,
		Currency:              head.Currency,
		TotalAmountCents:      head.TotalAmountCents,
		ExplainabilityVersion: "v1",
		LineItems:             lines,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func normalizeJSONMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		out[key] = normalizeJSONValue(v)
	}
	return out
}

func normalizeJSONValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return normalizeJSONMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeJSONValue(item))
		}
		return out
	default:
		return typed
	}
}

func parseTimeLoose(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC()
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}

func firstTimePointer(values ...string) *time.Time {
	for _, raw := range values {
		parsed := parseTimeLoose(raw)
		if !parsed.IsZero() {
			v := parsed
			return &v
		}
	}
	return nil
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringFromAny(v any) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
