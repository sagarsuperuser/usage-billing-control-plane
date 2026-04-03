package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) CreateReplayJob(input domain.ReplayJob) (domain.ReplayJob, error) {
	if input.ID == "" {
		input.ID = newID("rpl")
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	input.TenantID = normalizeTenantID(input.TenantID)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO replay_jobs (
			id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status,
			attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		input.ID,
		input.TenantID,
		nullableString(input.CustomerID),
		nullableString(input.MeterID),
		input.From,
		input.To,
		input.IdempotencyKey,
		string(input.Status),
		input.AttemptCount,
		input.LastAttemptAt,
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
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}

	return input, nil
}

func (s *PostgresStore) GetReplayJob(tenantID, id string) (domain.ReplayJob, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at FROM replay_jobs WHERE tenant_id = $1 AND id = $2`, normalizeTenantID(tenantID), id)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) GetReplayJobByIdempotencyKey(tenantID, key string) (domain.ReplayJob, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tenantID = normalizeTenantID(tenantID)
	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(ctx, `SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at FROM replay_jobs WHERE tenant_id = $1 AND idempotency_key = $2`, tenantID, key)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) ListReplayJobs(filter ReplayJobListFilter) (ReplayJobListResult, error) {
	tenantID := normalizeTenantID(filter.TenantID)
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	cursorID := strings.TrimSpace(filter.CursorID)
	cursorCreated := filter.CursorCreated

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return ReplayJobListResult{}, err
	}
	defer rollbackSilently(tx)

	baseClauses := []string{"tenant_id = $1"}
	baseArgs := []any{tenantID}
	nextArg := 2

	if customerID := strings.TrimSpace(filter.CustomerID); customerID != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("customer_id = $%d", nextArg))
		baseArgs = append(baseArgs, customerID)
		nextArg++
	}
	if meterID := strings.TrimSpace(filter.MeterID); meterID != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("meter_id = $%d", nextArg))
		baseArgs = append(baseArgs, meterID)
		nextArg++
	}
	if status := strings.ToLower(strings.TrimSpace(filter.Status)); status != "" {
		baseClauses = append(baseClauses, fmt.Sprintf("status = $%d", nextArg))
		baseArgs = append(baseArgs, status)
		nextArg++
	}

	baseWhere := strings.Join(baseClauses, " AND ")

	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM replay_jobs WHERE `+baseWhere, baseArgs...).Scan(&total); err != nil {
		return ReplayJobListResult{}, err
	}

	listClauses := append([]string{}, baseClauses...)
	listArgs := append([]any{}, baseArgs...)
	listArgPos := len(listArgs) + 1
	useCursor := cursorCreated != nil && cursorID != ""
	if useCursor {
		listClauses = append(listClauses, fmt.Sprintf("(created_at < $%d OR (created_at = $%d AND id < $%d))", listArgPos, listArgPos, listArgPos+1))
		listArgs = append(listArgs, *cursorCreated, cursorID)
		listArgPos += 2
	}
	listWhere := strings.Join(listClauses, " AND ")
	limitWithPeek := limit + 1

	var (
		query     string
		queryArgs []any
	)
	if useCursor {
		query = fmt.Sprintf(`SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at
			FROM replay_jobs
			WHERE %s
			ORDER BY created_at DESC, id DESC
			LIMIT $%d`, listWhere, listArgPos)
		queryArgs = append(listArgs, limitWithPeek)
	} else {
		query = fmt.Sprintf(`SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at
			FROM replay_jobs
			WHERE %s
			ORDER BY created_at DESC, id DESC
			LIMIT $%d OFFSET $%d`, listWhere, listArgPos, listArgPos+1)
		queryArgs = append(listArgs, limitWithPeek, offset)
	}

	rows, err := tx.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return ReplayJobListResult{}, err
	}
	defer rows.Close()

	items := make([]domain.ReplayJob, 0, limit)
	for rows.Next() {
		item, scanErr := scanReplayJob(rows)
		if scanErr != nil {
			return ReplayJobListResult{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ReplayJobListResult{}, err
	}

	var (
		nextCursorCreated *time.Time
		nextCursorID      string
	)
	hasMore := len(items) > limit
	if len(items) > limit {
		items = items[:limit]
	}
	if len(items) == limit && hasMore {
		last := items[len(items)-1]
		t := last.CreatedAt.UTC()
		nextCursorCreated = &t
		nextCursorID = last.ID
	}

	if err := tx.Commit(); err != nil {
		return ReplayJobListResult{}, err
	}
	return ReplayJobListResult{
		Items:             items,
		Total:             total,
		Limit:             limit,
		Offset:            offset,
		NextCursorID:      nextCursorID,
		NextCursorCreated: nextCursorCreated,
	}, nil
}

func (s *PostgresStore) RetryReplayJob(tenantID, id string) (domain.ReplayJob, error) {
	tenantID = normalizeTenantID(tenantID)
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.ReplayJob{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	currentRow := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at
		FROM replay_jobs
		WHERE tenant_id = $1 AND id = $2`,
		tenantID,
		id,
	)
	current, err := scanReplayJob(currentRow)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	if current.Status != domain.ReplayFailed {
		return domain.ReplayJob{}, fmt.Errorf("validation error: replay job can be retried only when status=failed")
	}

	row := tx.QueryRowContext(
		ctx,
		`UPDATE replay_jobs
		SET status = $1, error = '', started_at = NULL, completed_at = NULL, processed_records = 0
		WHERE tenant_id = $2 AND id = $3
		RETURNING id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at`,
		string(domain.ReplayQueued),
		tenantID,
		id,
	)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReplayJob{}, ErrNotFound
		}
		return domain.ReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) StartReplayJob(tenantID, id string) (domain.ReplayJob, error) {
	tenantID = normalizeTenantID(tenantID)
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.ReplayJob{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	now := time.Now().UTC()
	row := tx.QueryRowContext(
		ctx,
		`UPDATE replay_jobs
		SET status = $1, started_at = $2, last_attempt_at = $2, attempt_count = attempt_count + 1, processed_records = 0, error = '', completed_at = NULL
		WHERE tenant_id = $3 AND id = $4 AND status = $5
		RETURNING id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at`,
		string(domain.ReplayRunning),
		now,
		tenantID,
		id,
		string(domain.ReplayQueued),
	)
	job, err := scanReplayJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var currentStatus string
			statusErr := tx.QueryRowContext(ctx, `SELECT status FROM replay_jobs WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&currentStatus)
			if errors.Is(statusErr, sql.ErrNoRows) {
				return domain.ReplayJob{}, ErrNotFound
			}
			if statusErr != nil {
				return domain.ReplayJob{}, statusErr
			}
			return domain.ReplayJob{}, fmt.Errorf("%w: replay job can be started only when status=queued", ErrInvalidState)
		}
		return domain.ReplayJob{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) ListQueuedReplayJobs(limit int) ([]domain.ReplayJob, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at
		FROM replay_jobs
		WHERE status = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2`,
		string(domain.ReplayQueued),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ReplayJob, 0, limit)
	for rows.Next() {
		item, scanErr := scanReplayJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *PostgresStore) CompleteReplayJob(id string, processedRecords int64, completedAt time.Time) (domain.ReplayJob, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE replay_jobs SET status = $1, processed_records = $2, error = '', completed_at = $3 WHERE id = $4 RETURNING id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at`,
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
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}

func (s *PostgresStore) FailReplayJob(id string, errMessage string, completedAt time.Time) (domain.ReplayJob, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionBypass, "")
	if err != nil {
		return domain.ReplayJob{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`UPDATE replay_jobs SET status = $1, error = $2, completed_at = $3 WHERE id = $4 RETURNING id, tenant_id, customer_id, meter_id, from_ts, to_ts, idempotency_key, status, attempt_count, last_attempt_at, processed_records, error, created_at, started_at, completed_at`,
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
	if err := tx.Commit(); err != nil {
		return domain.ReplayJob{}, err
	}
	return job, nil
}


func scanReplayJob(s rowScanner) (domain.ReplayJob, error) {
	var out domain.ReplayJob
	var tenantID string
	var customerID sql.NullString
	var meterID sql.NullString
	var from sql.NullTime
	var to sql.NullTime
	var status string
	var lastAttemptAt sql.NullTime
	var startedAt sql.NullTime
	var completedAt sql.NullTime

	if err := s.Scan(
		&out.ID,
		&tenantID,
		&customerID,
		&meterID,
		&from,
		&to,
		&out.IdempotencyKey,
		&status,
		&out.AttemptCount,
		&lastAttemptAt,
		&out.ProcessedRecords,
		&out.Error,
		&out.CreatedAt,
		&startedAt,
		&completedAt,
	); err != nil {
		return domain.ReplayJob{}, err
	}

	out.TenantID = normalizeTenantID(tenantID)
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
	if lastAttemptAt.Valid {
		t := lastAttemptAt.Time.UTC()
		out.LastAttemptAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time.UTC()
		out.CompletedAt = &t
	}
	out.Status = domain.ReplayJobStatus(status)
	return out, nil
}

