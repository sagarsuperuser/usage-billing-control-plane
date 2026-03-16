package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

const (
	maxWebhookListLimit         = 500
	defaultLifecycleEventLimit  = 200
	maxSummaryStaleAfterSeconds = 7 * 24 * 60 * 60
)

func validateWebhookListWindow(limit, offset int) (int, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > maxWebhookListLimit {
		return 0, 0, fmt.Errorf("%w: limit must be between 1 and %d", ErrValidation, maxWebhookListLimit)
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("%w: offset must be >= 0", ErrValidation)
	}
	return limit, offset, nil
}

func buildInvoicePaymentLifecycle(view domain.InvoicePaymentStatusView, events []domain.LagoWebhookEvent, eventLimit int) InvoicePaymentLifecycle {
	out := InvoicePaymentLifecycle{
		TenantID:             strings.TrimSpace(view.TenantID),
		OrganizationID:       strings.TrimSpace(view.OrganizationID),
		InvoiceID:            strings.TrimSpace(view.InvoiceID),
		InvoiceStatus:        strings.TrimSpace(view.InvoiceStatus),
		PaymentStatus:        strings.ToLower(strings.TrimSpace(view.PaymentStatus)),
		PaymentOverdue:       view.PaymentOverdue,
		LastPaymentError:     strings.TrimSpace(view.LastPaymentError),
		LastEventType:        strings.TrimSpace(view.LastEventType),
		EventsAnalyzed:       len(events),
		EventWindowLimit:     eventLimit,
		EventWindowTruncated: len(events) >= eventLimit,
	}
	if !view.LastEventAt.IsZero() {
		last := view.LastEventAt.UTC()
		out.LastEventAt = &last
	}
	if !view.UpdatedAt.IsZero() {
		updated := view.UpdatedAt.UTC()
		out.UpdatedAt = &updated
	}

	webhookTypes := make(map[string]struct{}, len(events))
	for _, event := range events {
		webhookType := strings.ToLower(strings.TrimSpace(event.WebhookType))
		paymentStatus := strings.ToLower(strings.TrimSpace(event.PaymentStatus))
		if webhookType != "" {
			webhookTypes[webhookType] = struct{}{}
		}
		ts := event.OccurredAt.UTC()

		if webhookType == "invoice.payment_failure" || paymentStatus == "failed" {
			out.FailureEventCount++
			setLatestLifecycleTime(&out.LastFailureAt, ts)
		}
		if paymentStatus == "succeeded" {
			out.SuccessEventCount++
			setLatestLifecycleTime(&out.LastSuccessAt, ts)
		}
		if paymentStatus == "pending" {
			out.PendingEventCount++
			setLatestLifecycleTime(&out.LastPendingAt, ts)
		}
		if (event.PaymentOverdue != nil && *event.PaymentOverdue) || webhookType == "invoice.payment_overdue" {
			out.OverdueSignalCount++
			setLatestLifecycleTime(&out.LastOverdueAt, ts)
		}
	}
	out.DistinctWebhookTypes = make([]string, 0, len(webhookTypes))
	for webhookType := range webhookTypes {
		out.DistinctWebhookTypes = append(out.DistinctWebhookTypes, webhookType)
	}
	sort.Strings(out.DistinctWebhookTypes)

	switch out.PaymentStatus {
	case "succeeded":
		out.RecommendedAction = "none"
		out.RecommendedActionNote = "Payment succeeded. No collection action required."
	case "pending":
		out.RecommendedAction = "monitor_processing"
		out.RecommendedActionNote = "Payment is in progress. Monitor timeline for terminal state."
	case "failed":
		out.RequiresAction = true
		out.RetryRecommended = true
		out.RecommendedAction = "retry_payment"
		out.RecommendedActionNote = "Payment failed. Trigger retry-payment and verify customer funding method."
	default:
		out.RecommendedAction = "investigate"
		out.RecommendedActionNote = "Payment state is not terminal. Inspect webhook timeline for anomalies."
	}

	if out.PaymentOverdue != nil && *out.PaymentOverdue {
		out.RequiresAction = true
		if out.RecommendedAction == "none" || out.RecommendedAction == "monitor_processing" || out.RecommendedAction == "investigate" {
			out.RecommendedAction = "collect_payment"
			out.RecommendedActionNote = "Invoice is overdue. Start collection follow-up or dunning workflow."
		}
	}
	if out.RecommendedAction == "retry_payment" {
		out.RetryRecommended = true
	}

	return out
}

func setLatestLifecycleTime(dst **time.Time, candidate time.Time) {
	if candidate.IsZero() {
		return
	}
	candidate = candidate.UTC()
	if *dst == nil || candidate.After((**dst).UTC()) {
		v := candidate
		*dst = &v
	}
}

func normalizeWebhookListOrder(raw string) (bool, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" || v == "desc" {
		return true, nil
	}
	if v == "asc" {
		return false, nil
	}
	return false, fmt.Errorf("%w: order must be asc or desc", ErrValidation)
}

func normalizeInvoicePaymentStatusSortBy(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "last_event_at", nil
	}
	switch v {
	case "last_event_at", "updated_at", "total_due_amount_cents", "total_amount_cents":
		return v, nil
	default:
		return "", fmt.Errorf("%w: sort_by must be one of last_event_at, updated_at, total_due_amount_cents, total_amount_cents", ErrValidation)
	}
}

func normalizeLagoWebhookEventSortBy(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "received_at", nil
	}
	switch v {
	case "received_at", "occurred_at":
		return v, nil
	default:
		return "", fmt.Errorf("%w: sort_by must be one of received_at, occurred_at", ErrValidation)
	}
}

func buildWebhookKey(event domain.LagoWebhookEvent) string {
	base := strings.Join([]string{
		strings.TrimSpace(event.OrganizationID),
		strings.TrimSpace(event.WebhookType),
		strings.TrimSpace(event.ObjectType),
		strings.TrimSpace(event.InvoiceID),
		strings.TrimSpace(event.PaymentRequestID),
		strings.TrimSpace(event.DunningCampaignCode),
		strconv.FormatInt(event.OccurredAt.UnixNano(), 10),
	}, ":")
	if strings.Trim(base, ":") == "" {
		return fmt.Sprintf("generated:%d", time.Now().UTC().UnixNano())
	}
	return base
}

func parseLagoWebhookEnvelope(body []byte) (domain.LagoWebhookEvent, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.LagoWebhookEvent{}, fmt.Errorf("%w: invalid webhook payload", ErrValidation)
	}
	var envelope struct {
		WebhookType    string `json:"webhook_type"`
		ObjectType     string `json:"object_type"`
		OrganizationID string `json:"organization_id"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return domain.LagoWebhookEvent{}, fmt.Errorf("%w: invalid webhook envelope", ErrValidation)
	}
	envelope.WebhookType = strings.TrimSpace(envelope.WebhookType)
	envelope.ObjectType = strings.TrimSpace(envelope.ObjectType)
	envelope.OrganizationID = strings.TrimSpace(envelope.OrganizationID)
	if envelope.WebhookType == "" || envelope.ObjectType == "" || envelope.OrganizationID == "" {
		return domain.LagoWebhookEvent{}, fmt.Errorf("%w: webhook_type, object_type, and organization_id are required", ErrValidation)
	}

	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return domain.LagoWebhookEvent{}, fmt.Errorf("%w: invalid webhook payload", ErrValidation)
	}
	objectPayload := map[string]any{}
	if objectRaw, ok := raw[envelope.ObjectType]; ok && len(objectRaw) > 0 {
		_ = json.Unmarshal(objectRaw, &objectPayload)
	}

	event := domain.LagoWebhookEvent{
		OrganizationID: envelope.OrganizationID,
		WebhookType:    envelope.WebhookType,
		ObjectType:     envelope.ObjectType,
		Payload:        payload,
		ReceivedAt:     time.Now().UTC(),
		OccurredAt:     time.Now().UTC(),
	}
	populateLagoWebhookDerivedFields(&event, objectPayload)
	return event, nil
}

func populateLagoWebhookDerivedFields(event *domain.LagoWebhookEvent, objectPayload map[string]any) {
	if event == nil {
		return
	}

	switch event.WebhookType {
	case "invoice.payment_status_updated", "invoice.payment_overdue":
		event.InvoiceID = stringValue(objectPayload["lago_id"])
		event.InvoiceStatus = stringValue(objectPayload["status"])
		event.PaymentStatus = stringValue(objectPayload["payment_status"])
		event.PaymentOverdue = boolPtr(objectPayload["payment_overdue"])
		event.InvoiceNumber = stringValue(objectPayload["number"])
		event.Currency = stringValue(objectPayload["currency"])
		event.TotalAmountCents = int64Ptr(objectPayload["total_amount_cents"])
		event.TotalDueAmountCents = int64Ptr(objectPayload["total_due_amount_cents"])
		event.TotalPaidAmountCents = int64Ptr(objectPayload["total_paid_amount_cents"])
		if customer, ok := objectPayload["customer"].(map[string]any); ok {
			event.CustomerExternalID = stringValue(customer["external_id"])
		}
		event.OccurredAt = firstTimestamp(objectPayload["updated_at"], objectPayload["created_at"], event.ReceivedAt)

	case "invoice.payment_failure":
		event.InvoiceID = stringValue(objectPayload["lago_invoice_id"])
		event.CustomerExternalID = stringValue(objectPayload["external_customer_id"])
		event.PaymentStatus = "failed"
		event.LastPaymentError = stringValue(objectPayload["provider_error"])
		event.OccurredAt = event.ReceivedAt

	case "payment_request.payment_status_updated":
		event.PaymentRequestID = stringValue(objectPayload["lago_id"])
		event.PaymentStatus = stringValue(objectPayload["payment_status"])
		if invoices, ok := objectPayload["invoices"].([]any); ok && len(invoices) > 0 {
			if inv, ok := invoices[0].(map[string]any); ok {
				event.InvoiceID = stringValue(inv["lago_id"])
				event.InvoiceStatus = stringValue(inv["status"])
				if event.Currency == "" {
					event.Currency = stringValue(inv["currency"])
				}
				if event.PaymentStatus == "" {
					event.PaymentStatus = stringValue(inv["payment_status"])
				}
			}
		}
		if customer, ok := objectPayload["customer"].(map[string]any); ok {
			event.CustomerExternalID = stringValue(customer["external_id"])
		}
		event.OccurredAt = firstTimestamp(objectPayload["created_at"], nil, event.ReceivedAt)

	case "dunning_campaign.finished":
		event.DunningCampaignCode = stringValue(objectPayload["dunning_campaign_code"])
		event.CustomerExternalID = stringValue(objectPayload["external_customer_id"])
		if event.CustomerExternalID == "" {
			event.CustomerExternalID = stringValue(objectPayload["customer_external_id"])
		}
		event.OccurredAt = event.ReceivedAt

	default:
		event.OccurredAt = event.ReceivedAt
	}
}

func stringValue(v any) string {
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func boolPtr(v any) *bool {
	switch typed := v.(type) {
	case bool:
		out := typed
		return &out
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return nil
		}
		out := parsed
		return &out
	default:
		return nil
	}
}

func int64Ptr(v any) *int64 {
	switch typed := v.(type) {
	case int64:
		out := typed
		return &out
	case int:
		out := int64(typed)
		return &out
	case float64:
		out := int64(typed)
		return &out
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return nil
		}
		out := parsed
		return &out
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return nil
		}
		out := parsed
		return &out
	default:
		return nil
	}
}

func firstTimestamp(values ...any) time.Time {
	for _, raw := range values {
		switch typed := raw.(type) {
		case string:
			typed = strings.TrimSpace(typed)
			if typed == "" {
				continue
			}
			if ts, err := time.Parse(time.RFC3339, typed); err == nil {
				return ts.UTC()
			}
			if ts, err := time.Parse(time.RFC3339Nano, typed); err == nil {
				return ts.UTC()
			}
		case time.Time:
			return typed.UTC()
		}
	}
	return time.Now().UTC()
}
