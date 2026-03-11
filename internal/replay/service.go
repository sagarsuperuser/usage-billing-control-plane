package replay

import (
	"fmt"
	"strings"
	"time"

	"lago-usage-billing-alpha/internal/domain"
	"lago-usage-billing-alpha/internal/store"
)

type Service struct {
	store *store.MemoryStore
}

type CreateReplayJobRequest struct {
	CustomerID     string     `json:"customer_id"`
	MeterID        string     `json:"meter_id"`
	From           *time.Time `json:"from"`
	To             *time.Time `json:"to"`
	IdempotencyKey string     `json:"idempotency_key"`
}

func NewService(s *store.MemoryStore) *Service {
	return &Service{store: s}
}

func (s *Service) CreateJob(req CreateReplayJobRequest) (domain.ReplayJob, bool, error) {
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return domain.ReplayJob{}, false, fmt.Errorf("idempotency_key is required")
	}

	if req.MeterID != "" {
		if _, err := s.store.GetMeter(req.MeterID); err != nil {
			return domain.ReplayJob{}, false, fmt.Errorf("meter_id not found")
		}
	}

	if req.From != nil && req.To != nil && req.From.After(*req.To) {
		return domain.ReplayJob{}, false, fmt.Errorf("from must be <= to")
	}

	job := domain.ReplayJob{
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
			existing, getErr := s.store.GetReplayJobByIdempotencyKey(req.IdempotencyKey)
			if getErr != nil {
				return domain.ReplayJob{}, false, getErr
			}
			return existing, true, nil
		}
		return domain.ReplayJob{}, false, err
	}

	go s.process(created.ID)
	return created, false, nil
}

func (s *Service) process(jobID string) {
	now := time.Now().UTC()
	_, _ = s.store.UpdateReplayJob(jobID, func(job domain.ReplayJob) domain.ReplayJob {
		job.Status = domain.ReplayRunning
		job.StartedAt = &now
		return job
	})

	job, err := s.store.GetReplayJob(jobID)
	if err != nil {
		return
	}

	events := s.store.ListUsageEvents(job.From, job.To, job.CustomerID, job.MeterID)

	completed := time.Now().UTC()
	_, _ = s.store.UpdateReplayJob(jobID, func(job domain.ReplayJob) domain.ReplayJob {
		job.Status = domain.ReplayDone
		job.ProcessedRecords = int64(len(events))
		job.CompletedAt = &completed
		return job
	})
}

func (s *Service) GetJob(id string) (domain.ReplayJob, error) {
	return s.store.GetReplayJob(id)
}
