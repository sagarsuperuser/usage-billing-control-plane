package replay

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lago-usage-billing-alpha/internal/domain"
	"lago-usage-billing-alpha/internal/store"
)

type Service struct {
	store store.Repository
}

type CreateReplayJobRequest struct {
	TenantID       string     `json:"tenant_id,omitempty"`
	CustomerID     string     `json:"customer_id"`
	MeterID        string     `json:"meter_id"`
	From           *time.Time `json:"from"`
	To             *time.Time `json:"to"`
	IdempotencyKey string     `json:"idempotency_key"`
}

type ListReplayJobsRequest struct {
	CustomerID string `json:"customer_id,omitempty"`
	MeterID    string `json:"meter_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
}

type ListReplayJobsResult struct {
	Items      []domain.ReplayJob `json:"items"`
	Total      int                `json:"total"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

type ReplayJobDiagnostics struct {
	Job                domain.ReplayJob `json:"job"`
	UsageEventsCount   int              `json:"usage_events_count"`
	UsageQuantity      int64            `json:"usage_quantity"`
	BilledEntriesCount int              `json:"billed_entries_count"`
	BilledAmountCents  int64            `json:"billed_amount_cents"`
}

type replayListCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func NewService(s store.Repository) *Service {
	return &Service{store: s}
}

func (s *Service) CreateJob(req CreateReplayJobRequest) (domain.ReplayJob, bool, error) {
	req.TenantID = normalizeTenantID(req.TenantID)
	req.CustomerID = strings.TrimSpace(req.CustomerID)
	req.MeterID = strings.TrimSpace(req.MeterID)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if req.IdempotencyKey == "" {
		return domain.ReplayJob{}, false, fmt.Errorf("idempotency_key is required")
	}

	if req.MeterID != "" {
		meter, err := s.store.GetMeter(req.TenantID, req.MeterID)
		if err != nil {
			return domain.ReplayJob{}, false, fmt.Errorf("meter_id not found")
		}
		if normalizeTenantID(meter.TenantID) != req.TenantID {
			return domain.ReplayJob{}, false, fmt.Errorf("meter_id not found")
		}
	}

	if req.From != nil && req.To != nil && req.From.After(*req.To) {
		return domain.ReplayJob{}, false, fmt.Errorf("from must be <= to")
	}

	job := domain.ReplayJob{
		TenantID:         req.TenantID,
		CustomerID:       req.CustomerID,
		MeterID:          req.MeterID,
		From:             req.From,
		To:               req.To,
		IdempotencyKey:   req.IdempotencyKey,
		Status:           domain.ReplayQueued,
		ProcessedRecords: 0,
	}

	created, err := s.store.CreateReplayJob(job)
	if err != nil {
		if err == store.ErrAlreadyExists {
			existing, getErr := s.store.GetReplayJobByIdempotencyKey(req.TenantID, req.IdempotencyKey)
			if getErr != nil {
				return domain.ReplayJob{}, false, getErr
			}
			if !isReplayJobIdempotencyMatch(existing, req) {
				return domain.ReplayJob{}, false, fmt.Errorf("%w: idempotency_key already used with different payload", store.ErrDuplicateKey)
			}
			return existing, true, nil
		}
		return domain.ReplayJob{}, false, err
	}

	return created, false, nil
}

func (s *Service) GetJob(tenantID, id string) (domain.ReplayJob, error) {
	return s.store.GetReplayJob(normalizeTenantID(tenantID), id)
}

func (s *Service) RetryJob(tenantID, id string) (domain.ReplayJob, error) {
	return s.store.RetryReplayJob(normalizeTenantID(tenantID), strings.TrimSpace(id))
}

func (s *Service) ListJobs(tenantID string, req ListReplayJobsRequest) (ListReplayJobsResult, error) {
	limit, offset, err := normalizeReplayListWindow(req.Limit, req.Offset)
	if err != nil {
		return ListReplayJobsResult{}, err
	}
	status, err := normalizeReplayStatusFilter(req.Status)
	if err != nil {
		return ListReplayJobsResult{}, err
	}
	cursorCreated, cursorID, err := decodeReplayCursor(req.Cursor)
	if err != nil {
		return ListReplayJobsResult{}, err
	}
	if cursorCreated != nil && offset > 0 {
		return ListReplayJobsResult{}, fmt.Errorf("validation error: offset cannot be used with cursor")
	}

	out, err := s.store.ListReplayJobs(store.ReplayJobListFilter{
		TenantID:      normalizeTenantID(tenantID),
		CustomerID:    strings.TrimSpace(req.CustomerID),
		MeterID:       strings.TrimSpace(req.MeterID),
		Status:        status,
		Limit:         limit,
		Offset:        offset,
		CursorID:      cursorID,
		CursorCreated: cursorCreated,
	})
	if err != nil {
		return ListReplayJobsResult{}, err
	}
	nextCursor, err := encodeReplayCursor(out.NextCursorCreated, out.NextCursorID)
	if err != nil {
		return ListReplayJobsResult{}, err
	}
	return ListReplayJobsResult{
		Items:      out.Items,
		Total:      out.Total,
		Limit:      out.Limit,
		Offset:     out.Offset,
		NextCursor: nextCursor,
	}, nil
}

func (s *Service) GetJobDiagnostics(tenantID, id string) (ReplayJobDiagnostics, error) {
	tenantID = normalizeTenantID(tenantID)
	job, err := s.store.GetReplayJob(tenantID, strings.TrimSpace(id))
	if err != nil {
		return ReplayJobDiagnostics{}, err
	}

	filter := store.Filter{
		TenantID:   tenantID,
		From:       job.From,
		To:         job.To,
		CustomerID: strings.TrimSpace(job.CustomerID),
		MeterID:    strings.TrimSpace(job.MeterID),
	}
	usageEvents, err := s.store.ListUsageEvents(filter)
	if err != nil {
		return ReplayJobDiagnostics{}, err
	}
	billedEntries, err := s.store.ListBilledEntries(filter)
	if err != nil {
		return ReplayJobDiagnostics{}, err
	}

	out := ReplayJobDiagnostics{
		Job:                job,
		UsageEventsCount:   len(usageEvents),
		BilledEntriesCount: len(billedEntries),
	}
	for _, event := range usageEvents {
		out.UsageQuantity += event.Quantity
	}
	for _, entry := range billedEntries {
		out.BilledAmountCents += entry.AmountCents
	}
	return out, nil
}

func normalizeTenantID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "default"
	}
	return v
}

func normalizeReplayStatusFilter(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "":
		return "", nil
	case string(domain.ReplayQueued), string(domain.ReplayRunning), string(domain.ReplayDone), string(domain.ReplayFailed):
		return raw, nil
	default:
		return "", fmt.Errorf("validation error: invalid status filter")
	}
}

func normalizeReplayListWindow(limit, offset int) (int, int, error) {
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return 0, 0, fmt.Errorf("validation error: limit must be between 1 and 100")
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("validation error: offset must be >= 0")
	}
	return limit, offset, nil
}

func decodeReplayCursor(raw string) (*time.Time, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", fmt.Errorf("validation error: invalid cursor")
	}
	var c replayListCursor
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, "", fmt.Errorf("validation error: invalid cursor")
	}
	c.ID = strings.TrimSpace(c.ID)
	if c.ID == "" || c.CreatedAt.IsZero() {
		return nil, "", fmt.Errorf("validation error: invalid cursor")
	}
	t := c.CreatedAt.UTC()
	return &t, c.ID, nil
}

func encodeReplayCursor(createdAt *time.Time, id string) (string, error) {
	id = strings.TrimSpace(id)
	if createdAt == nil || id == "" {
		return "", nil
	}
	payload, err := json.Marshal(replayListCursor{
		CreatedAt: createdAt.UTC(),
		ID:        id,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func isReplayJobIdempotencyMatch(existing domain.ReplayJob, requested CreateReplayJobRequest) bool {
	if strings.TrimSpace(existing.CustomerID) != strings.TrimSpace(requested.CustomerID) {
		return false
	}
	if strings.TrimSpace(existing.MeterID) != strings.TrimSpace(requested.MeterID) {
		return false
	}
	return equalOptionalTime(existing.From, requested.From) && equalOptionalTime(existing.To, requested.To)
}

func equalOptionalTime(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.UTC().Equal(b.UTC())
}
