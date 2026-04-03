package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"usage-billing-control-plane/internal/domain"
)

func (s *PostgresStore) CreateUsageEvent(input domain.UsageEvent) (domain.UsageEvent, error) {
	if input.ID == "" {
		input.ID = newID("evt")
	}
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now().UTC()
	}
	input.TenantID = normalizeTenantID(input.TenantID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.UsageEvent{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO usage_events (id, tenant_id, customer_id, meter_id, subscription_id, quantity, idempotency_key, occurred_at) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),$8)`,
		input.ID,
		input.TenantID,
		input.CustomerID,
		input.MeterID,
		strings.TrimSpace(input.SubscriptionID),
		input.Quantity,
		input.IdempotencyKey,
		input.Timestamp,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.UsageEvent{}, ErrAlreadyExists
		}
		return domain.UsageEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.UsageEvent{}, err
	}
	return input, nil
}

func (s *PostgresStore) GetUsageEventByIdempotencyKey(tenantID, idempotencyKey string) (domain.UsageEvent, error) {
	tenantID = normalizeTenantID(tenantID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return domain.UsageEvent{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.UsageEvent{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, customer_id, meter_id, subscription_id, quantity, idempotency_key, occurred_at
		FROM usage_events
		WHERE tenant_id = $1 AND idempotency_key = $2
		ORDER BY occurred_at DESC, id DESC
		LIMIT 1`,
		tenantID,
		idempotencyKey,
	)
	var event domain.UsageEvent
	var subscriptionID sql.NullString
	var eventIdempotencyKey sql.NullString
	if err := row.Scan(
		&event.ID,
		&event.TenantID,
		&event.CustomerID,
		&event.MeterID,
		&subscriptionID,
		&event.Quantity,
		&eventIdempotencyKey,
		&event.Timestamp,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.UsageEvent{}, ErrNotFound
		}
		return domain.UsageEvent{}, err
	}
	event.TenantID = normalizeTenantID(event.TenantID)
	if subscriptionID.Valid {
		event.SubscriptionID = strings.TrimSpace(subscriptionID.String)
	}
	if eventIdempotencyKey.Valid {
		event.IdempotencyKey = strings.TrimSpace(eventIdempotencyKey.String)
	}

	if err := tx.Commit(); err != nil {
		return domain.UsageEvent{}, err
	}
	return event, nil
}

func (s *PostgresStore) ListUsageEvents(filter Filter) ([]domain.UsageEvent, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, filter.TenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	query, args := buildUsageEventsFilteredQuery(`SELECT id, tenant_id, customer_id, meter_id, subscription_id, quantity, idempotency_key, occurred_at FROM usage_events`, filter, "occurred_at")
	if filter.SortDesc {
		query += ` ORDER BY occurred_at DESC, id DESC`
	} else {
		query += ` ORDER BY occurred_at ASC, id ASC`
	}
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.UsageEvent, 0)
	for rows.Next() {
		var event domain.UsageEvent
		var subscriptionID sql.NullString
		var idempotencyKey sql.NullString
		if scanErr := rows.Scan(&event.ID, &event.TenantID, &event.CustomerID, &event.MeterID, &subscriptionID, &event.Quantity, &idempotencyKey, &event.Timestamp); scanErr != nil {
			return nil, scanErr
		}
		event.TenantID = normalizeTenantID(event.TenantID)
		if subscriptionID.Valid {
			event.SubscriptionID = strings.TrimSpace(subscriptionID.String)
		}
		if idempotencyKey.Valid {
			event.IdempotencyKey = strings.TrimSpace(idempotencyKey.String)
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
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
	input.TenantID = normalizeTenantID(input.TenantID)
	input.Source = normalizeBilledEntrySource(input.Source)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.ReplayJobID = strings.TrimSpace(input.ReplayJobID)

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, input.TenantID)
	if err != nil {
		return domain.BilledEntry{}, err
	}
	defer rollbackSilently(tx)

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO billed_entries (id, tenant_id, customer_id, meter_id, amount_cents, idempotency_key, source, replay_job_id, occurred_at) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9)`,
		input.ID,
		input.TenantID,
		input.CustomerID,
		input.MeterID,
		input.AmountCents,
		input.IdempotencyKey,
		string(input.Source),
		nullableString(input.ReplayJobID),
		input.Timestamp,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.BilledEntry{}, ErrAlreadyExists
		}
		return domain.BilledEntry{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.BilledEntry{}, err
	}
	return input, nil
}

func (s *PostgresStore) GetBilledEntryByIdempotencyKey(tenantID, idempotencyKey string) (domain.BilledEntry, error) {
	tenantID = normalizeTenantID(tenantID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return domain.BilledEntry{}, ErrNotFound
	}

	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return domain.BilledEntry{}, err
	}
	defer rollbackSilently(tx)

	row := tx.QueryRowContext(
		ctx,
		`SELECT id, tenant_id, customer_id, meter_id, amount_cents, idempotency_key, source, replay_job_id, occurred_at
		FROM billed_entries
		WHERE tenant_id = $1 AND idempotency_key = $2
		ORDER BY occurred_at DESC, id DESC
		LIMIT 1`,
		tenantID,
		idempotencyKey,
	)
	var entry domain.BilledEntry
	var source string
	var entryIdempotencyKey sql.NullString
	var replayJobID sql.NullString
	if err := row.Scan(
		&entry.ID,
		&entry.TenantID,
		&entry.CustomerID,
		&entry.MeterID,
		&entry.AmountCents,
		&entryIdempotencyKey,
		&source,
		&replayJobID,
		&entry.Timestamp,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.BilledEntry{}, ErrNotFound
		}
		return domain.BilledEntry{}, err
	}
	entry.TenantID = normalizeTenantID(entry.TenantID)
	entry.Source = normalizeBilledEntrySource(domain.BilledEntrySource(source))
	if entryIdempotencyKey.Valid {
		entry.IdempotencyKey = strings.TrimSpace(entryIdempotencyKey.String)
	}
	if replayJobID.Valid {
		entry.ReplayJobID = strings.TrimSpace(replayJobID.String)
	}

	if err := tx.Commit(); err != nil {
		return domain.BilledEntry{}, err
	}
	return entry, nil
}

func (s *PostgresStore) ListBilledEntries(filter Filter) ([]domain.BilledEntry, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, filter.TenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	query, args := buildBilledEntriesFilteredQuery(`SELECT id, tenant_id, customer_id, meter_id, amount_cents, idempotency_key, source, replay_job_id, occurred_at FROM billed_entries`, filter, "occurred_at")
	if filter.SortDesc {
		query += ` ORDER BY occurred_at DESC, id DESC`
	} else {
		query += ` ORDER BY occurred_at ASC, id ASC`
	}
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.BilledEntry, 0)
	for rows.Next() {
		var entry domain.BilledEntry
		var idempotencyKey sql.NullString
		var source string
		var replayJobID sql.NullString
		if scanErr := rows.Scan(&entry.ID, &entry.TenantID, &entry.CustomerID, &entry.MeterID, &entry.AmountCents, &idempotencyKey, &source, &replayJobID, &entry.Timestamp); scanErr != nil {
			return nil, scanErr
		}
		entry.TenantID = normalizeTenantID(entry.TenantID)
		entry.Source = normalizeBilledEntrySource(domain.BilledEntrySource(source))
		if idempotencyKey.Valid {
			entry.IdempotencyKey = strings.TrimSpace(idempotencyKey.String)
		}
		if replayJobID.Valid {
			entry.ReplayJobID = strings.TrimSpace(replayJobID.String)
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}


func (s *PostgresStore) AggregateUsageForBillingPeriod(tenantID, subscriptionID string, meterIDs []string, from, to time.Time) (map[string]int64, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	tx, err := s.beginTxWithSession(ctx, txSessionTenant, tenantID)
	if err != nil {
		return nil, err
	}
	defer rollbackSilently(tx)

	rows, err := tx.QueryContext(ctx, `
		SELECT meter_id, COALESCE(SUM(quantity), 0) AS total_quantity
		FROM usage_events
		WHERE subscription_id = $1
			AND meter_id = ANY($2)
			AND occurred_at >= $3
			AND occurred_at < $4
		GROUP BY meter_id`, subscriptionID, pq.Array(meterIDs), from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var meterID string
		var total int64
		if err := rows.Scan(&meterID, &total); err != nil {
			return nil, err
		}
		result[meterID] = total
	}
	return result, rows.Err()
}

// ---------------------------------------------------------------------------
// Stripe webhook events
// ---------------------------------------------------------------------------


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
	if strings.TrimSpace(filter.TenantID) != "" {
		add("tenant_id = $%d", normalizeTenantID(filter.TenantID))
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

func buildUsageEventsFilteredQuery(base string, filter Filter, timeColumn string) (string, []any) {
	clauses := make([]string, 0, 7)
	args := make([]any, 0, 7)

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
	if strings.TrimSpace(filter.TenantID) != "" {
		add("tenant_id = $%d", normalizeTenantID(filter.TenantID))
	}
	if filter.CursorOccurredAt != nil && strings.TrimSpace(filter.CursorID) != "" {
		args = append(args, filter.CursorOccurredAt.UTC(), strings.TrimSpace(filter.CursorID))
		posOccurredAt := len(args) - 1
		posID := len(args)
		if filter.SortDesc {
			clauses = append(clauses, fmt.Sprintf("(%s < $%d OR (%s = $%d AND id < $%d))", timeColumn, posOccurredAt, timeColumn, posOccurredAt, posID))
		} else {
			clauses = append(clauses, fmt.Sprintf("(%s > $%d OR (%s = $%d AND id > $%d))", timeColumn, posOccurredAt, timeColumn, posOccurredAt, posID))
		}
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

func buildBilledEntriesFilteredQuery(base string, filter Filter, timeColumn string) (string, []any) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)

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
	if strings.TrimSpace(filter.TenantID) != "" {
		add("tenant_id = $%d", normalizeTenantID(filter.TenantID))
	}
	if strings.TrimSpace(string(filter.BilledSource)) != "" {
		add("source = $%d", strings.TrimSpace(string(filter.BilledSource)))
	}
	if strings.TrimSpace(filter.BilledReplayJobID) != "" {
		add("replay_job_id = $%d", strings.TrimSpace(filter.BilledReplayJobID))
	}
	if filter.CursorOccurredAt != nil && strings.TrimSpace(filter.CursorID) != "" {
		args = append(args, filter.CursorOccurredAt.UTC(), strings.TrimSpace(filter.CursorID))
		posOccurredAt := len(args) - 1
		posID := len(args)
		if filter.SortDesc {
			clauses = append(clauses, fmt.Sprintf("(%s < $%d OR (%s = $%d AND id < $%d))", timeColumn, posOccurredAt, timeColumn, posOccurredAt, posID))
		} else {
			clauses = append(clauses, fmt.Sprintf("(%s > $%d OR (%s = $%d AND id > $%d))", timeColumn, posOccurredAt, timeColumn, posOccurredAt, posID))
		}
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


func normalizeBilledEntrySource(v domain.BilledEntrySource) domain.BilledEntrySource {
	normalized := domain.BilledEntrySource(strings.TrimSpace(string(v)))
	if normalized == "" {
		return domain.BilledEntrySourceAPI
	}
	return normalized
}

