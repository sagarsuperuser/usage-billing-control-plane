package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type AuditExportService struct {
	store          store.Repository
	apiKeyService  *APIKeyService
	objectStore    ObjectStore
	downloadExpiry time.Duration
}

type CreateAuditExportJobRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	APIKeyID       string `json:"api_key_id,omitempty"`
	ActorAPIKeyID  string `json:"actor_api_key_id,omitempty"`
	Action         string `json:"action,omitempty"`
	OwnerType      string `json:"owner_type,omitempty"`
	OwnerID        string `json:"owner_id,omitempty"`
}

type AuditExportJobResponse struct {
	Job         domain.APIKeyAuditExportJob `json:"job"`
	DownloadURL string                      `json:"download_url,omitempty"`
}

type ListAuditExportJobsRequest struct {
	Status              string `json:"status,omitempty"`
	RequestedByAPIKeyID string `json:"requested_by_api_key_id,omitempty"`
	OwnerType           string `json:"owner_type,omitempty"`
	OwnerID             string `json:"owner_id,omitempty"`
	Limit               int    `json:"limit,omitempty"`
	Offset              int    `json:"offset,omitempty"`
	Cursor              string `json:"cursor,omitempty"`
}

type ListAuditExportJobsResult struct {
	Items      []AuditExportJobResponse `json:"items"`
	Total      int                      `json:"total"`
	Limit      int                      `json:"limit"`
	Offset     int                      `json:"offset"`
	NextCursor string                   `json:"next_cursor,omitempty"`
}

type auditExportListCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func NewAuditExportService(repo store.Repository, objectStore ObjectStore, downloadExpiry time.Duration) *AuditExportService {
	if downloadExpiry <= 0 {
		downloadExpiry = 24 * time.Hour
	}
	return &AuditExportService{
		store:          repo,
		apiKeyService:  NewAPIKeyService(repo),
		objectStore:    objectStore,
		downloadExpiry: downloadExpiry,
	}
}

func (s *AuditExportService) CreateJob(tenantID, actorAPIKeyID string, req CreateAuditExportJobRequest) (domain.APIKeyAuditExportJob, bool, error) {
	tenantID = normalizeTenantID(tenantID)
	actorAPIKeyID = strings.TrimSpace(actorAPIKeyID)

	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		return domain.APIKeyAuditExportJob{}, false, fmt.Errorf("%w: idempotency_key is required", ErrValidation)
	}
	action, err := normalizeAuditAction(req.Action)
	if err != nil {
		return domain.APIKeyAuditExportJob{}, false, err
	}

	job := domain.APIKeyAuditExportJob{
		TenantID:            tenantID,
		RequestedByAPIKeyID: actorAPIKeyID,
		IdempotencyKey:      idempotencyKey,
		Status:              domain.APIKeyAuditExportQueued,
		Filters: domain.APIKeyAuditExportFilters{
			APIKeyID:      strings.TrimSpace(req.APIKeyID),
			ActorAPIKeyID: strings.TrimSpace(req.ActorAPIKeyID),
			Action:        action,
			OwnerType:     strings.TrimSpace(req.OwnerType),
			OwnerID:       strings.TrimSpace(req.OwnerID),
		},
	}

	created, err := s.store.CreateAPIKeyAuditExportJob(job)
	if err != nil {
		if err == store.ErrAlreadyExists {
			existing, getErr := s.store.GetAPIKeyAuditExportJobByIdempotencyKey(tenantID, idempotencyKey)
			if getErr != nil {
				return domain.APIKeyAuditExportJob{}, false, getErr
			}
			return existing, true, nil
		}
		return domain.APIKeyAuditExportJob{}, false, err
	}
	return created, false, nil
}

func (s *AuditExportService) GetJob(tenantID, id string) (AuditExportJobResponse, error) {
	tenantID = normalizeTenantID(tenantID)
	job, err := s.store.GetAPIKeyAuditExportJob(tenantID, id)
	if err != nil {
		return AuditExportJobResponse{}, err
	}
	url, err := s.presignedDownloadURL(context.Background(), job)
	if err != nil {
		return AuditExportJobResponse{}, err
	}
	return AuditExportJobResponse{
		Job:         job,
		DownloadURL: url,
	}, nil
}

func (s *AuditExportService) ListJobs(tenantID string, req ListAuditExportJobsRequest) (ListAuditExportJobsResult, error) {
	limit, offset, err := normalizeAuditExportListWindow(req.Limit, req.Offset)
	if err != nil {
		return ListAuditExportJobsResult{}, err
	}
	status, err := normalizeAuditExportStatus(req.Status)
	if err != nil {
		return ListAuditExportJobsResult{}, err
	}
	cursorCreated, cursorID, err := decodeAuditExportCursor(req.Cursor)
	if err != nil {
		return ListAuditExportJobsResult{}, err
	}
	if cursorCreated != nil && offset > 0 {
		return ListAuditExportJobsResult{}, fmt.Errorf("%w: offset cannot be used with cursor", ErrValidation)
	}

	out, err := s.store.ListAPIKeyAuditExportJobs(store.APIKeyAuditExportFilter{
		TenantID:            normalizeTenantID(tenantID),
		Status:              status,
		RequestedByAPIKeyID: strings.TrimSpace(req.RequestedByAPIKeyID),
		OwnerType:           strings.TrimSpace(req.OwnerType),
		OwnerID:             strings.TrimSpace(req.OwnerID),
		Limit:               limit,
		Offset:              offset,
		CursorID:            cursorID,
		CursorCreated:       cursorCreated,
	})
	if err != nil {
		return ListAuditExportJobsResult{}, err
	}

	items := make([]AuditExportJobResponse, 0, len(out.Items))
	for _, job := range out.Items {
		url, presignErr := s.presignedDownloadURL(context.Background(), job)
		if presignErr != nil {
			return ListAuditExportJobsResult{}, presignErr
		}
		items = append(items, AuditExportJobResponse{
			Job:         job,
			DownloadURL: url,
		})
	}

	nextCursor, err := encodeAuditExportCursor(out.NextCursorCreated, out.NextCursorID)
	if err != nil {
		return ListAuditExportJobsResult{}, err
	}
	return ListAuditExportJobsResult{
		Items:      items,
		Total:      out.Total,
		Limit:      out.Limit,
		Offset:     out.Offset,
		NextCursor: nextCursor,
	}, nil
}

func (s *AuditExportService) ProcessNext(ctx context.Context) (bool, error) {
	if s.objectStore == nil {
		return false, nil
	}
	job, err := s.store.DequeueAPIKeyAuditExportJob()
	if err != nil {
		if err == store.ErrNotFound {
			return false, nil
		}
		return false, err
	}

	csvData, err := s.apiKeyService.GenerateAPIKeyAuditCSV(job.TenantID, ListAPIKeyAuditEventsRequest{
		APIKeyID:      job.Filters.APIKeyID,
		ActorAPIKeyID: job.Filters.ActorAPIKeyID,
		Action:        job.Filters.Action,
		OwnerType:     job.Filters.OwnerType,
		OwnerID:       job.Filters.OwnerID,
		Limit:         maxListLimit,
	})
	if err != nil {
		_, _ = s.store.FailAPIKeyAuditExportJob(job.ID, err.Error(), time.Now().UTC())
		return true, nil
	}

	objectKey := path.Join("tenants", job.TenantID, "audit-exports", job.ID+".csv")
	if err := s.objectStore.PutObject(ctx, objectKey, []byte(csvData), "text/csv; charset=utf-8"); err != nil {
		_, _ = s.store.FailAPIKeyAuditExportJob(job.ID, err.Error(), time.Now().UTC())
		return true, nil
	}

	rowCount := int64(0)
	trimmed := strings.TrimSpace(csvData)
	if trimmed != "" {
		lines := strings.Count(trimmed, "\n")
		if lines > 0 {
			rowCount = int64(lines)
		}
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.downloadExpiry)
	if _, err := s.store.CompleteAPIKeyAuditExportJob(job.ID, objectKey, rowCount, now, expiresAt); err != nil {
		_, _ = s.store.FailAPIKeyAuditExportJob(job.ID, err.Error(), time.Now().UTC())
		return true, nil
	}
	return true, nil
}

func (s *AuditExportService) presignedDownloadURL(ctx context.Context, job domain.APIKeyAuditExportJob) (string, error) {
	if s.objectStore == nil {
		return "", nil
	}
	if job.Status != domain.APIKeyAuditExportDone {
		return "", nil
	}
	if strings.TrimSpace(job.ObjectKey) == "" {
		return "", nil
	}
	if job.ExpiresAt != nil && time.Now().UTC().After(job.ExpiresAt.UTC()) {
		return "", nil
	}

	expiry := 5 * time.Minute
	if job.ExpiresAt != nil {
		until := time.Until(job.ExpiresAt.UTC())
		if until <= 0 {
			return "", nil
		}
		if until < expiry {
			expiry = until
		}
	}
	return s.objectStore.PresignGetObject(ctx, job.ObjectKey, expiry)
}

type AuditExportWorker struct {
	service         *AuditExportService
	pollInterval    time.Duration
	errorBackoff    time.Duration
	maxErrorBackoff time.Duration
}

func NewAuditExportWorker(s *AuditExportService, pollInterval time.Duration) *AuditExportWorker {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	return &AuditExportWorker{
		service:         s,
		pollInterval:    pollInterval,
		errorBackoff:    250 * time.Millisecond,
		maxErrorBackoff: 5 * time.Second,
	}
}

func (w *AuditExportWorker) Run(ctx context.Context) {
	if w == nil || w.service == nil {
		return
	}
	backoff := w.errorBackoff
	for {
		processed, err := w.service.ProcessNext(ctx)
		if err != nil {
			if !sleepWithContext(ctx, backoff) {
				return
			}
			backoff *= 2
			if backoff > w.maxErrorBackoff {
				backoff = w.maxErrorBackoff
			}
			continue
		}
		backoff = w.errorBackoff
		if processed {
			select {
			case <-ctx.Done():
				return
			default:
			}
			continue
		}
		if !sleepWithContext(ctx, w.pollInterval) {
			return
		}
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func normalizeAuditExportStatus(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "":
		return "", nil
	case string(domain.APIKeyAuditExportQueued), string(domain.APIKeyAuditExportRunning), string(domain.APIKeyAuditExportDone), string(domain.APIKeyAuditExportFailed):
		return raw, nil
	default:
		return "", fmt.Errorf("%w: invalid status filter", ErrValidation)
	}
}

func normalizeAuditExportListWindow(limit, offset int) (int, int, error) {
	if limit == 0 {
		limit = defaultListLimit
	}
	if limit < 1 || limit > maxListLimit {
		return 0, 0, fmt.Errorf("%w: limit must be between 1 and %d", ErrValidation, maxListLimit)
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("%w: offset must be >= 0", ErrValidation)
	}
	return limit, offset, nil
}

func decodeAuditExportCursor(raw string) (*time.Time, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	var c auditExportListCursor
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	c.ID = strings.TrimSpace(c.ID)
	if c.ID == "" || c.CreatedAt.IsZero() {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	t := c.CreatedAt.UTC()
	return &t, c.ID, nil
}

func encodeAuditExportCursor(createdAt *time.Time, id string) (string, error) {
	id = strings.TrimSpace(id)
	if createdAt == nil || id == "" {
		return "", nil
	}
	payload, err := json.Marshal(auditExportListCursor{
		CreatedAt: createdAt.UTC(),
		ID:        id,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}
