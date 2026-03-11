package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"lago-usage-billing-alpha/internal/domain"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS rating_rule_versions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			version INTEGER NOT NULL,
			mode TEXT NOT NULL,
			currency TEXT NOT NULL,
			flat_amount_cents BIGINT NOT NULL DEFAULT 0,
			graduated_tiers JSONB NOT NULL DEFAULT '[]'::jsonb,
			package_size BIGINT NOT NULL DEFAULT 0,
			package_amount_cents BIGINT NOT NULL DEFAULT 0,
			overage_unit_amount_cents BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS meters (
			id TEXT PRIMARY KEY,
			meter_key TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			unit TEXT NOT NULL,
			aggregation TEXT NOT NULL,
			rating_rule_version_id TEXT NOT NULL REFERENCES rating_rule_versions(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS usage_events (
			id TEXT PRIMARY KEY,
			customer_id TEXT NOT NULL,
			meter_id TEXT NOT NULL REFERENCES meters(id),
			quantity BIGINT NOT NULL,
			occurred_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_lookup ON usage_events (customer_id, meter_id, occurred_at)`,
		`CREATE TABLE IF NOT EXISTS billed_entries (
			id TEXT PRIMARY KEY,
			customer_id TEXT NOT NULL,
			meter_id TEXT NOT NULL REFERENCES meters(id),
			amount_cents BIGINT NOT NULL,
			occurred_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_billed_entries_lookup ON billed_entries (customer_id, meter_id, occurred_at)`,
		`CREATE TABLE IF NOT EXISTS replay_jobs (
			id TEXT PRIMARY KEY,
			customer_id TEXT,
			meter_id TEXT,
			from_ts TIMESTAMPTZ,
			to_ts TIMESTAMPTZ,
			idempotency_key TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL,
			processed_records BIGINT NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			started_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_replay_jobs_status_created ON replay_jobs (status, created_at)`,
	}

	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) CreateRatingRuleVersion(input domain.RatingRuleVersion) (domain.RatingRuleVersion, error) {
	if input.ID == "" {
		input.ID = newID("rrv")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	if input.GraduatedTiers == nil {
		input.GraduatedTiers = []domain.RatingTier{}
	}

	tiers, err := json.Marshal(input.GraduatedTiers)
	if err != nil {
		return domain.RatingRuleVersion{}, err
	}

	_, err = s.db.Exec(
		`INSERT INTO rating_rule_versions (
			id, name, version, mode, currency, flat_amount_cents, graduated_tiers,
			package_size, package_amount_cents, overage_unit_amount_cents, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11)`,
		input.ID,
		input.Name,
		input.Version,
		string(input.Mode),
		input.Currency,
		input.FlatAmountCents,
		string(tiers),
		input.PackageSize,
		input.PackageAmountCents,
		input.OverageUnitAmountCents,
		input.CreatedAt,
	)
	if err != nil {
		return domain.RatingRuleVersion{}, err
	}

	return input, nil
}

func (s *PostgresStore) ListRatingRuleVersions() ([]domain.RatingRuleVersion, error) {
	rows, err := s.db.Query(`SELECT id, name, version, mode, currency, flat_amount_cents, graduated_tiers, package_size, package_amount_cents, overage_unit_amount_cents, created_at FROM rating_rule_versions ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.RatingRuleVersion, 0)
	for rows.Next() {
		rule, scanErr := scanRatingRule(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) GetRatingRuleVersion(id string) (domain.RatingRuleVersion, error) {
	row := s.db.QueryRow(`SELECT id, name, version, mode, currency, flat_amount_cents, graduated_tiers, package_size, package_amount_cents, overage_unit_amount_cents, created_at FROM rating_rule_versions WHERE id = $1`, id)
	rule, err := scanRatingRule(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.RatingRuleVersion{}, ErrNotFound
		}
		return domain.RatingRuleVersion{}, err
	}
	return rule, nil
}

func (s *PostgresStore) CreateMeter(input domain.Meter) (domain.Meter, error) {
	if input.ID == "" {
		input.ID = newID("mtr")
	}
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.UpdatedAt = now

	_, err := s.db.Exec(
		`INSERT INTO meters (id, meter_key, name, unit, aggregation, rating_rule_version_id, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		input.ID,
		input.Key,
		input.Name,
		input.Unit,
		input.Aggregation,
		input.RatingRuleVersionID,
		input.CreatedAt,
		input.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Meter{}, ErrDuplicateKey
		}
		return domain.Meter{}, err
	}

	return input, nil
}

func (s *PostgresStore) ListMeters() ([]domain.Meter, error) {
	rows, err := s.db.Query(`SELECT id, meter_key, name, unit, aggregation, rating_rule_version_id, created_at, updated_at FROM meters ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Meter, 0)
	for rows.Next() {
		meter, scanErr := scanMeter(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, meter)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) GetMeter(id string) (domain.Meter, error) {
	row := s.db.QueryRow(`SELECT id, meter_key, name, unit, aggregation, rating_rule_version_id, created_at, updated_at FROM meters WHERE id = $1`, id)
	meter, err := scanMeter(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Meter{}, ErrNotFound
		}
		return domain.Meter{}, err
	}
	return meter, nil
}

func (s *PostgresStore) UpdateMeter(input domain.Meter) (domain.Meter, error) {
	input.UpdatedAt = time.Now().UTC()

	row := s.db.QueryRow(
		`UPDATE meters SET meter_key = $1, name = $2, unit = $3, aggregation = $4, rating_rule_version_id = $5, updated_at = $6 WHERE id = $7 RETURNING id, meter_key, name, unit, aggregation, rating_rule_version_id, created_at, updated_at`,
		input.Key,
		input.Name,
		input.Unit,
		input.Aggregation,
		input.RatingRuleVersionID,
		input.UpdatedAt,
		input.ID,
	)

	meter, err := scanMeter(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Meter{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return domain.Meter{}, ErrDuplicateKey
		}
		return domain.Meter{}, err
	}

	return meter, nil
}

func (s *PostgresStore) CreateUsageEvent(input domain.UsageEvent) (domain.UsageEvent, error) {
	if input.ID == "" {
		input.ID = newID("evt")
	}
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now().UTC()
	}

	_, err := s.db.Exec(`INSERT INTO usage_events (id, customer_id, meter_id, quantity, occurred_at) VALUES ($1,$2,$3,$4,$5)`, input.ID, input.CustomerID, input.MeterID, input.Quantity, input.Timestamp)
	if err != nil {
		return domain.UsageEvent{}, err
	}
	return input, nil
}

func (s *PostgresStore) ListUsageEvents(filter Filter) ([]domain.UsageEvent, error) {
	query, args := buildFilteredQuery(`SELECT id, customer_id, meter_id, quantity, occurred_at FROM usage_events`, filter, "occurred_at")
	query += ` ORDER BY occurred_at ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.UsageEvent, 0)
	for rows.Next() {
		var event domain.UsageEvent
		if scanErr := rows.Scan(&event.ID, &event.CustomerID, &event.MeterID, &event.Quantity, &event.Timestamp); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) CreateBilledEntry(input domain.BilledEntry) (domain.BilledEntry, error) {
	if input.ID == "" {
		input.ID = newID("bil")
	}
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now().UTC()
	}

	_, err := s.db.Exec(`INSERT INTO billed_entries (id, customer_id, meter_id, amount_cents, occurred_at) VALUES ($1,$2,$3,$4,$5)`, input.ID, input.CustomerID, input.MeterID, input.AmountCents, input.Timestamp)
	if err != nil {
		return domain.BilledEntry{}, err
	}
	return input, nil
}

func (s *PostgresStore) ListBilledEntries(filter Filter) ([]domain.BilledEntry, error) {
	query, args := buildFilteredQuery(`SELECT id, customer_id, meter_id, amount_cents, occurred_at FROM billed_entries`, filter, "occurred_at")
	query += ` ORDER BY occurred_at ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.BilledEntry, 0)
	for rows.Next() {
		var entry domain.BilledEntry
		if scanErr := rows.Scan(&entry.ID, &entry.CustomerID, &entry.MeterID, &entry.AmountCents, &entry.Timestamp); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresStore) CreateReplayJob(input domain.ReplayJob) (domain.ReplayJob, error) {
	if input.ID == "" {
		input.ID = newID("rpl")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.Exec(
		`INSERT INTO replay_jobs (id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, processed_records, error, created_at, started_at, completed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		input.ID,
		nullableString(input.CustomerID),
		nullableString(input.MeterID),
		input.From,
		input.To,
		input.IdempotencyKey,
		string(input.Status),
		input.ProcessedRecords,
		input.Error,
		input.CreatedAt,
		input.StartedAt,
		input.CompletedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ReplayJob{}, ErrAlreadyExists
		}
		return domain.ReplayJob{}, err
	}

	return input, nil
}

func (s *PostgresStore) GetReplayJob(id string) (domain.ReplayJob, error) {
	row := s.db.QueryRow(`SELECT id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, processed_records, error, created_at, started_at, completed_at FROM replay_jobs WHERE id = $1`, id)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) GetReplayJobByIdempotencyKey(key string) (domain.ReplayJob, error) {
	row := s.db.QueryRow(`SELECT id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, processed_records, error, created_at, started_at, completed_at FROM replay_jobs WHERE idempotency_key = $1`, key)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) DequeueReplayJob() (domain.ReplayJob, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	row := tx.QueryRow(`SELECT id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, processed_records, error, created_at, started_at, completed_at FROM replay_jobs WHERE status = $1 ORDER BY created_at ASC FOR UPDATE SKIP LOCKED LIMIT 1`, string(domain.ReplayQueued))
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}

	now := time.Now().UTC()
	_, err = tx.Exec(`UPDATE replay_jobs SET status = $1, started_at = $2 WHERE id = $3`, string(domain.ReplayRunning), now, job.ID)
	if err != nil {
		return domain.ReplayJob{}, err
	}

	job.Status = domain.ReplayRunning
	job.StartedAt = &now

	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) CompleteReplayJob(id string, processedRecords int64, completedAt time.Time) (domain.ReplayJob, error) {
	row := s.db.QueryRow(
		`UPDATE replay_jobs SET status = $1, processed_records = $2, error = '', completed_at = $3 WHERE id = $4 RETURNING id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, processed_records, error, created_at, started_at, completed_at`,
		string(domain.ReplayDone),
		processedRecords,
		completedAt,
		id,
	)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) FailReplayJob(id string, errMessage string, completedAt time.Time) (domain.ReplayJob, error) {
	row := s.db.QueryRow(
		`UPDATE replay_jobs SET status = $1, error = $2, completed_at = $3 WHERE id = $4 RETURNING id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, processed_records, error, created_at, started_at, completed_at`,
		string(domain.ReplayFailed),
		errMessage,
		completedAt,
		id,
	)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	return job, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRatingRule(s rowScanner) (domain.RatingRuleVersion, error) {
	var out domain.RatingRuleVersion
	var mode string
	var tiersRaw []byte
	if err := s.Scan(
		&out.ID,
		&out.Name,
		&out.Version,
		&mode,
		&out.Currency,
		&out.FlatAmountCents,
		&tiersRaw,
		&out.PackageSize,
		&out.PackageAmountCents,
		&out.OverageUnitAmountCents,
		&out.CreatedAt,
	); err != nil {
		return domain.RatingRuleVersion{}, err
	}
	out.Mode = domain.PricingMode(mode)
	if len(tiersRaw) == 0 {
		out.GraduatedTiers = []domain.RatingTier{}
		return out, nil
	}
	if err := json.Unmarshal(tiersRaw, &out.GraduatedTiers); err != nil {
		return domain.RatingRuleVersion{}, err
	}
	return out, nil
}

func scanMeter(s rowScanner) (domain.Meter, error) {
	var out domain.Meter
	if err := s.Scan(
		&out.ID,
		&out.Key,
		&out.Name,
		&out.Unit,
		&out.Aggregation,
		&out.RatingRuleVersionID,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return domain.Meter{}, err
	}
	return out, nil
}

func scanReplayJob(s rowScanner) (domain.ReplayJob, error) {
	var out domain.ReplayJob
	var customerID sql.NullString
	var meterID sql.NullString
	var from sql.NullTime
	var to sql.NullTime
	var status string
	var startedAt sql.NullTime
	var completedAt sql.NullTime

	if err := s.Scan(
		&out.ID,
		&customerID,
		&meterID,
		&from,
		&to,
		&out.IdempotencyKey,
		&status,
		&out.ProcessedRecords,
		&out.Error,
		&out.CreatedAt,
		&startedAt,
		&completedAt,
	); err != nil {
		return domain.ReplayJob{}, err
	}

	if customerID.Valid {
		out.CustomerID = customerID.String
	}
	if meterID.Valid {
		out.MeterID = meterID.String
	}
	if from.Valid {
		t := from.Time.UTC()
		out.From = &t
	}
	if to.Valid {
		t := to.Time.UTC()
		out.To = &t
	}
	if startedAt.Valid {
		t := startedAt.Time.UTC()
		out.StartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time.UTC()
		out.CompletedAt = &t
	}
	out.Status = domain.ReplayJobStatus(status)
	return out, nil
}

func buildFilteredQuery(base string, filter Filter, timeColumn string) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)

	add := func(format string, val any) {
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf(format, len(args)))
	}

	if filter.CustomerID != "" {
		add("customer_id = $%d", filter.CustomerID)
	}
	if filter.MeterID != "" {
		add("meter_id = $%d", filter.MeterID)
	}
	if filter.From != nil {
		add(timeColumn+" >= $%d", *filter.From)
	}
	if filter.To != nil {
		add(timeColumn+" <= $%d", *filter.To)
	}

	if len(clauses) > 0 {
		base += " WHERE " + strings.Join(clauses, " AND ")
	}

	return base, args
}

func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(buf))
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "duplicate key value") || strings.Contains(text, "unique constraint")
}
