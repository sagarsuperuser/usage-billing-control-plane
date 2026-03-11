package store

import (
	"errors"
	"time"

	"lago-usage-billing-alpha/internal/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrDuplicateKey  = errors.New("duplicate key")
)

type Filter struct {
	From       *time.Time
	To         *time.Time
	CustomerID string
	MeterID    string
}

type Repository interface {
	Migrate() error

	CreateRatingRuleVersion(input domain.RatingRuleVersion) (domain.RatingRuleVersion, error)
	ListRatingRuleVersions() ([]domain.RatingRuleVersion, error)
	GetRatingRuleVersion(id string) (domain.RatingRuleVersion, error)

	CreateMeter(input domain.Meter) (domain.Meter, error)
	ListMeters() ([]domain.Meter, error)
	GetMeter(id string) (domain.Meter, error)
	UpdateMeter(input domain.Meter) (domain.Meter, error)

	CreateUsageEvent(input domain.UsageEvent) (domain.UsageEvent, error)
	ListUsageEvents(filter Filter) ([]domain.UsageEvent, error)

	CreateBilledEntry(input domain.BilledEntry) (domain.BilledEntry, error)
	ListBilledEntries(filter Filter) ([]domain.BilledEntry, error)

	CreateReplayJob(input domain.ReplayJob) (domain.ReplayJob, error)
	GetReplayJob(id string) (domain.ReplayJob, error)
	GetReplayJobByIdempotencyKey(key string) (domain.ReplayJob, error)
	DequeueReplayJob() (domain.ReplayJob, error)
	CompleteReplayJob(id string, processedRecords int64, completedAt time.Time) (domain.ReplayJob, error)
	FailReplayJob(id string, errMessage string, completedAt time.Time) (domain.ReplayJob, error)
}
