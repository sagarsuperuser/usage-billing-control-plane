package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type UsageService struct {
	store store.Repository
}

type ListBilledEntriesRequest struct {
	CustomerID        string
	MeterID           string
	BilledSource      string
	BilledReplayJobID string
	Order             string
	From              *time.Time
	To                *time.Time
	Limit             int
	Offset            int
	Cursor            string
}

type ListBilledEntriesResult struct {
	Items      []domain.BilledEntry `json:"items"`
	Limit      int                  `json:"limit"`
	Offset     int                  `json:"offset"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

type ListUsageEventsRequest struct {
	CustomerID string
	MeterID    string
	Order      string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
	Cursor     string
}

type ListUsageEventsResult struct {
	Items      []domain.UsageEvent `json:"items"`
	Limit      int                 `json:"limit"`
	Offset     int                 `json:"offset"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

type billedEntriesCursor struct {
	OccurredAt time.Time `json:"occurred_at"`
	ID         string    `json:"id"`
}

type usageEventsCursor struct {
	OccurredAt time.Time `json:"occurred_at"`
	ID         string    `json:"id"`
}

func NewUsageService(s store.Repository) *UsageService {
	return &UsageService{store: s}
}

func (s *UsageService) CreateUsageEvent(input domain.UsageEvent) (domain.UsageEvent, error) {
	event, _, err := s.CreateUsageEventWithIdempotency(input)
	return event, err
}

func (s *UsageService) CreateUsageEventWithIdempotency(input domain.UsageEvent) (domain.UsageEvent, bool, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	if strings.TrimSpace(input.CustomerID) == "" || strings.TrimSpace(input.MeterID) == "" {
		return domain.UsageEvent{}, false, fmt.Errorf("%w: customer_id and meter_id are required", ErrValidation)
	}
	if input.Quantity < 0 {
		return domain.UsageEvent{}, false, fmt.Errorf("%w: quantity must be >= 0", ErrValidation)
	}
	meter, err := s.store.GetMeter(input.TenantID, input.MeterID)
	if err != nil {
		return domain.UsageEvent{}, false, fmt.Errorf("%w: meter not found", ErrValidation)
	}
	if normalizeTenantID(meter.TenantID) != input.TenantID {
		return domain.UsageEvent{}, false, fmt.Errorf("%w: meter tenant mismatch", ErrValidation)
	}

	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.IdempotencyKey != "" {
		existing, getErr := s.store.GetUsageEventByIdempotencyKey(input.TenantID, input.IdempotencyKey)
		if getErr == nil {
			if !isUsageEventIdempotencyMatch(existing, input) {
				return domain.UsageEvent{}, false, fmt.Errorf("%w: idempotency_key already used with different payload", store.ErrDuplicateKey)
			}
			return existing, true, nil
		}
		if getErr != store.ErrNotFound {
			return domain.UsageEvent{}, false, getErr
		}
	}

	created, err := s.store.CreateUsageEvent(input)
	if err != nil {
		if input.IdempotencyKey != "" && (err == store.ErrAlreadyExists || err == store.ErrDuplicateKey) {
			existing, getErr := s.store.GetUsageEventByIdempotencyKey(input.TenantID, input.IdempotencyKey)
			if getErr == nil {
				if !isUsageEventIdempotencyMatch(existing, input) {
					return domain.UsageEvent{}, false, fmt.Errorf("%w: idempotency_key already used with different payload", store.ErrDuplicateKey)
				}
				return existing, true, nil
			}
			if getErr != store.ErrNotFound {
				return domain.UsageEvent{}, false, getErr
			}
		}
		return domain.UsageEvent{}, false, err
	}
	return created, false, nil
}

func (s *UsageService) CreateBilledEntry(input domain.BilledEntry) (domain.BilledEntry, error) {
	entry, _, err := s.CreateBilledEntryWithIdempotency(input)
	return entry, err
}

func (s *UsageService) CreateBilledEntryWithIdempotency(input domain.BilledEntry) (domain.BilledEntry, bool, error) {
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Source = domain.BilledEntrySourceAPI
	input.ReplayJobID = ""
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if strings.TrimSpace(input.CustomerID) == "" || strings.TrimSpace(input.MeterID) == "" {
		return domain.BilledEntry{}, false, fmt.Errorf("%w: customer_id and meter_id are required", ErrValidation)
	}
	meter, err := s.store.GetMeter(input.TenantID, input.MeterID)
	if err != nil {
		return domain.BilledEntry{}, false, fmt.Errorf("%w: meter not found", ErrValidation)
	}
	if normalizeTenantID(meter.TenantID) != input.TenantID {
		return domain.BilledEntry{}, false, fmt.Errorf("%w: meter tenant mismatch", ErrValidation)
	}

	if input.IdempotencyKey != "" {
		existing, getErr := s.store.GetBilledEntryByIdempotencyKey(input.TenantID, input.IdempotencyKey)
		if getErr == nil {
			if !isBilledEntryIdempotencyMatch(existing, input) {
				return domain.BilledEntry{}, false, fmt.Errorf("%w: idempotency_key already used with different payload", store.ErrDuplicateKey)
			}
			return existing, true, nil
		}
		if getErr != store.ErrNotFound {
			return domain.BilledEntry{}, false, getErr
		}
	}

	created, err := s.store.CreateBilledEntry(input)
	if err != nil {
		if input.IdempotencyKey != "" && (err == store.ErrAlreadyExists || err == store.ErrDuplicateKey) {
			existing, getErr := s.store.GetBilledEntryByIdempotencyKey(input.TenantID, input.IdempotencyKey)
			if getErr == nil {
				if !isBilledEntryIdempotencyMatch(existing, input) {
					return domain.BilledEntry{}, false, fmt.Errorf("%w: idempotency_key already used with different payload", store.ErrDuplicateKey)
				}
				return existing, true, nil
			}
			if getErr != store.ErrNotFound {
				return domain.BilledEntry{}, false, getErr
			}
		}
		return domain.BilledEntry{}, false, err
	}
	return created, false, nil
}

func (s *UsageService) ListBilledEntries(tenantID string, req ListBilledEntriesRequest) (ListBilledEntriesResult, error) {
	tenantID = normalizeTenantID(tenantID)

	req.BilledSource = strings.ToLower(strings.TrimSpace(req.BilledSource))
	source := domain.BilledEntrySource(req.BilledSource)
	if source != "" {
		switch source {
		case domain.BilledEntrySourceAPI, domain.BilledEntrySourceReplayAdjustment:
		default:
			return ListBilledEntriesResult{}, fmt.Errorf("%w: billed_source must be api or replay_adjustment", ErrValidation)
		}
	}

	limit := req.Limit
	if req.Limit < 0 {
		return ListBilledEntriesResult{}, fmt.Errorf("%w: limit must be >= 0", ErrValidation)
	}
	if req.Offset < 0 {
		return ListBilledEntriesResult{}, fmt.Errorf("%w: offset must be >= 0", ErrValidation)
	}
	if limit == 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if req.From != nil && req.To != nil && req.From.After(*req.To) {
		return ListBilledEntriesResult{}, fmt.Errorf("%w: from must be <= to", ErrValidation)
	}
	sortDesc, err := normalizeListOrder(req.Order)
	if err != nil {
		return ListBilledEntriesResult{}, err
	}
	cursorOccurredAt, cursorID, err := decodeBilledEntriesCursor(req.Cursor)
	if err != nil {
		return ListBilledEntriesResult{}, err
	}
	if cursorOccurredAt != nil && req.Offset > 0 {
		return ListBilledEntriesResult{}, fmt.Errorf("%w: offset cannot be used with cursor", ErrValidation)
	}

	items, err := s.store.ListBilledEntries(store.Filter{
		TenantID:          tenantID,
		From:              req.From,
		To:                req.To,
		CustomerID:        strings.TrimSpace(req.CustomerID),
		MeterID:           strings.TrimSpace(req.MeterID),
		BilledSource:      source,
		BilledReplayJobID: strings.TrimSpace(req.BilledReplayJobID),
		SortDesc:          sortDesc,
		Limit:             limit + 1,
		Offset:            req.Offset,
		CursorID:          cursorID,
		CursorOccurredAt:  cursorOccurredAt,
	})
	if err != nil {
		return ListBilledEntriesResult{}, err
	}

	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor, err = encodeBilledEntriesCursor(&last.Timestamp, last.ID)
		if err != nil {
			return ListBilledEntriesResult{}, err
		}
	}

	return ListBilledEntriesResult{
		Items:      items,
		Limit:      limit,
		Offset:     req.Offset,
		NextCursor: nextCursor,
	}, nil
}

func (s *UsageService) ListUsageEvents(tenantID string, req ListUsageEventsRequest) (ListUsageEventsResult, error) {
	tenantID = normalizeTenantID(tenantID)

	limit := req.Limit
	if req.Limit < 0 {
		return ListUsageEventsResult{}, fmt.Errorf("%w: limit must be >= 0", ErrValidation)
	}
	if req.Offset < 0 {
		return ListUsageEventsResult{}, fmt.Errorf("%w: offset must be >= 0", ErrValidation)
	}
	if limit == 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if req.From != nil && req.To != nil && req.From.After(*req.To) {
		return ListUsageEventsResult{}, fmt.Errorf("%w: from must be <= to", ErrValidation)
	}
	sortDesc, err := normalizeListOrder(req.Order)
	if err != nil {
		return ListUsageEventsResult{}, err
	}

	cursorOccurredAt, cursorID, err := decodeUsageEventsCursor(req.Cursor)
	if err != nil {
		return ListUsageEventsResult{}, err
	}
	if cursorOccurredAt != nil && req.Offset > 0 {
		return ListUsageEventsResult{}, fmt.Errorf("%w: offset cannot be used with cursor", ErrValidation)
	}

	items, err := s.store.ListUsageEvents(store.Filter{
		TenantID:         tenantID,
		From:             req.From,
		To:               req.To,
		CustomerID:       strings.TrimSpace(req.CustomerID),
		MeterID:          strings.TrimSpace(req.MeterID),
		SortDesc:         sortDesc,
		Limit:            limit + 1,
		Offset:           req.Offset,
		CursorID:         cursorID,
		CursorOccurredAt: cursorOccurredAt,
	})
	if err != nil {
		return ListUsageEventsResult{}, err
	}

	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor, err = encodeUsageEventsCursor(&last.Timestamp, last.ID)
		if err != nil {
			return ListUsageEventsResult{}, err
		}
	}

	return ListUsageEventsResult{
		Items:      items,
		Limit:      limit,
		Offset:     req.Offset,
		NextCursor: nextCursor,
	}, nil
}

func decodeBilledEntriesCursor(raw string) (*time.Time, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	var c billedEntriesCursor
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	c.ID = strings.TrimSpace(c.ID)
	if c.ID == "" || c.OccurredAt.IsZero() {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	t := c.OccurredAt.UTC()
	return &t, c.ID, nil
}

func encodeBilledEntriesCursor(occurredAt *time.Time, id string) (string, error) {
	id = strings.TrimSpace(id)
	if occurredAt == nil || id == "" {
		return "", nil
	}
	payload, err := json.Marshal(billedEntriesCursor{
		OccurredAt: occurredAt.UTC(),
		ID:         id,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeUsageEventsCursor(raw string) (*time.Time, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	var c usageEventsCursor
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	c.ID = strings.TrimSpace(c.ID)
	if c.ID == "" || c.OccurredAt.IsZero() {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	t := c.OccurredAt.UTC()
	return &t, c.ID, nil
}

func encodeUsageEventsCursor(occurredAt *time.Time, id string) (string, error) {
	id = strings.TrimSpace(id)
	if occurredAt == nil || id == "" {
		return "", nil
	}
	payload, err := json.Marshal(usageEventsCursor{
		OccurredAt: occurredAt.UTC(),
		ID:         id,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func isUsageEventIdempotencyMatch(existing, requested domain.UsageEvent) bool {
	if strings.TrimSpace(existing.CustomerID) != strings.TrimSpace(requested.CustomerID) {
		return false
	}
	if strings.TrimSpace(existing.MeterID) != strings.TrimSpace(requested.MeterID) {
		return false
	}
	if existing.Quantity != requested.Quantity {
		return false
	}
	if !requested.Timestamp.IsZero() && !existing.Timestamp.UTC().Equal(requested.Timestamp.UTC()) {
		return false
	}
	return true
}

func isBilledEntryIdempotencyMatch(existing, requested domain.BilledEntry) bool {
	if strings.TrimSpace(existing.CustomerID) != strings.TrimSpace(requested.CustomerID) {
		return false
	}
	if strings.TrimSpace(existing.MeterID) != strings.TrimSpace(requested.MeterID) {
		return false
	}
	if existing.AmountCents != requested.AmountCents {
		return false
	}
	if !requested.Timestamp.IsZero() && !existing.Timestamp.UTC().Equal(requested.Timestamp.UTC()) {
		return false
	}
	return true
}

func normalizeListOrder(raw string) (bool, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "", "asc":
		return false, nil
	case "desc":
		return true, nil
	default:
		return false, fmt.Errorf("%w: order must be asc or desc", ErrValidation)
	}
}
