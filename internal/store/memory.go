package store

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"lago-usage-billing-alpha/internal/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrDuplicateKey  = errors.New("duplicate key")
)

type MemoryStore struct {
	mu sync.RWMutex

	ruleVersions    map[string]domain.RatingRuleVersion
	meters          map[string]domain.Meter
	usageEvents     map[string]domain.UsageEvent
	billedEntries   map[string]domain.BilledEntry
	replayJobs      map[string]domain.ReplayJob
	replayByIdemKey map[string]string
	meterByKey      map[string]string
	counters        map[string]int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		ruleVersions:    make(map[string]domain.RatingRuleVersion),
		meters:          make(map[string]domain.Meter),
		usageEvents:     make(map[string]domain.UsageEvent),
		billedEntries:   make(map[string]domain.BilledEntry),
		replayJobs:      make(map[string]domain.ReplayJob),
		replayByIdemKey: make(map[string]string),
		meterByKey:      make(map[string]string),
		counters:        make(map[string]int64),
	}
}

func (s *MemoryStore) nextID(prefix string) string {
	s.counters[prefix]++
	return fmt.Sprintf("%s_%06d", prefix, s.counters[prefix])
}

func (s *MemoryStore) CreateRatingRuleVersion(input domain.RatingRuleVersion) (domain.RatingRuleVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	input.ID = s.nextID("rrv")
	input.CreatedAt = time.Now().UTC()
	s.ruleVersions[input.ID] = input
	return input, nil
}

func (s *MemoryStore) ListRatingRuleVersions() []domain.RatingRuleVersion {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.RatingRuleVersion, 0, len(s.ruleVersions))
	for _, rule := range s.ruleVersions {
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (s *MemoryStore) GetRatingRuleVersion(id string) (domain.RatingRuleVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rule, ok := s.ruleVersions[id]
	if !ok {
		return domain.RatingRuleVersion{}, ErrNotFound
	}
	return rule, nil
}

func (s *MemoryStore) CreateMeter(input domain.Meter) (domain.Meter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.meterByKey[input.Key]; exists {
		return domain.Meter{}, ErrDuplicateKey
	}
	input.ID = s.nextID("mtr")
	now := time.Now().UTC()
	input.CreatedAt = now
	input.UpdatedAt = now
	s.meters[input.ID] = input
	s.meterByKey[input.Key] = input.ID
	return input, nil
}

func (s *MemoryStore) ListMeters() []domain.Meter {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.Meter, 0, len(s.meters))
	for _, meter := range s.meters {
		out = append(out, meter)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (s *MemoryStore) GetMeter(id string) (domain.Meter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meter, ok := s.meters[id]
	if !ok {
		return domain.Meter{}, ErrNotFound
	}
	return meter, nil
}

func (s *MemoryStore) UpdateMeter(id string, updateFn func(domain.Meter) (domain.Meter, error)) (domain.Meter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meter, ok := s.meters[id]
	if !ok {
		return domain.Meter{}, ErrNotFound
	}

	updated, err := updateFn(meter)
	if err != nil {
		return domain.Meter{}, err
	}

	if updated.Key != meter.Key {
		if _, exists := s.meterByKey[updated.Key]; exists {
			return domain.Meter{}, ErrDuplicateKey
		}
		delete(s.meterByKey, meter.Key)
		s.meterByKey[updated.Key] = id
	}

	updated.ID = id
	updated.CreatedAt = meter.CreatedAt
	updated.UpdatedAt = time.Now().UTC()
	s.meters[id] = updated
	return updated, nil
}

func (s *MemoryStore) CreateUsageEvent(input domain.UsageEvent) (domain.UsageEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	input.ID = s.nextID("evt")
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now().UTC()
	}
	s.usageEvents[input.ID] = input
	return input, nil
}

func (s *MemoryStore) ListUsageEvents(from, to *time.Time, customerID, meterID string) []domain.UsageEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.UsageEvent, 0)
	for _, event := range s.usageEvents {
		if customerID != "" && event.CustomerID != customerID {
			continue
		}
		if meterID != "" && event.MeterID != meterID {
			continue
		}
		if from != nil && event.Timestamp.Before(*from) {
			continue
		}
		if to != nil && event.Timestamp.After(*to) {
			continue
		}
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}

func (s *MemoryStore) CreateBilledEntry(input domain.BilledEntry) (domain.BilledEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	input.ID = s.nextID("bil")
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now().UTC()
	}
	s.billedEntries[input.ID] = input
	return input, nil
}

func (s *MemoryStore) ListBilledEntries(from, to *time.Time, customerID, meterID string) []domain.BilledEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.BilledEntry, 0)
	for _, entry := range s.billedEntries {
		if customerID != "" && entry.CustomerID != customerID {
			continue
		}
		if meterID != "" && entry.MeterID != meterID {
			continue
		}
		if from != nil && entry.Timestamp.Before(*from) {
			continue
		}
		if to != nil && entry.Timestamp.After(*to) {
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}

func (s *MemoryStore) CreateReplayJob(input domain.ReplayJob) (domain.ReplayJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if jobID, ok := s.replayByIdemKey[input.IdempotencyKey]; ok {
		job := s.replayJobs[jobID]
		return job, ErrAlreadyExists
	}

	input.ID = s.nextID("rpl")
	input.CreatedAt = time.Now().UTC()
	s.replayJobs[input.ID] = input
	s.replayByIdemKey[input.IdempotencyKey] = input.ID
	return input, nil
}

func (s *MemoryStore) UpdateReplayJob(id string, updateFn func(domain.ReplayJob) domain.ReplayJob) (domain.ReplayJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.replayJobs[id]
	if !ok {
		return domain.ReplayJob{}, ErrNotFound
	}
	job = updateFn(job)
	s.replayJobs[id] = job
	return job, nil
}

func (s *MemoryStore) GetReplayJob(id string) (domain.ReplayJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.replayJobs[id]
	if !ok {
		return domain.ReplayJob{}, ErrNotFound
	}
	return job, nil
}

func (s *MemoryStore) GetReplayJobByIdempotencyKey(key string) (domain.ReplayJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.replayByIdemKey[key]
	if !ok {
		return domain.ReplayJob{}, ErrNotFound
	}
	job := s.replayJobs[id]
	return job, nil
}
