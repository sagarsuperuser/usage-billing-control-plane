# Database Migration Standards

## Strategy: Forward-Only

We do not write down migrations. If a migration breaks, fix forward with a new migration. This is the same pattern used by Stripe, Vercel, and every serious billing platform.

**Why:**
- Down migrations for billing schemas lose data (DROP TABLE invoices = gone)
- Nobody runs down migrations in production
- Rollback safety comes from database backups (RDS snapshots), not reverse SQL
- Down migrations create a false sense of safety while adding maintenance burden

## Naming

```
NNNN_snake_case_description.up.sql
```

- 4-digit zero-padded version number
- Snake case description of what the migration does
- `.up.sql` suffix only (no `.down.sql`)
- Sequential, no gaps

Examples:
```
0053_add_invoice_memo_column.up.sql
0054_create_webhook_delivery_tracking.up.sql
```

## Rules

1. **Always use `IF NOT EXISTS` / `IF EXISTS`** — migrations must be idempotent
2. **Never modify a deployed migration** — create a new one instead
3. **One concern per migration** — don't combine unrelated schema changes
4. **Add indexes concurrently** — use `CREATE INDEX CONCURRENTLY` for large tables (requires running outside transaction: add `-- migrate:no-transaction` comment)
5. **Test locally before pushing** — run `make migrate` against local Postgres
6. **Include RLS policies** — every new table needs `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` and tenant-scoped policies

## Adding a New Migration

```bash
# Create the file
touch migrations/0053_your_description.up.sql

# Write your SQL
# Run locally
make migrate

# Verify
make migrate-status
make migrate-verify
```

## Deployment

Migrations run automatically via Helm pre-install/pre-upgrade hooks before application services start. The golang-migrate library handles:
- Exclusive locking (prevents concurrent runs)
- Transaction wrapping (each migration is atomic)
- Dirty state detection (fails fast if previous migration was interrupted)

## Rollback Strategy

If a migration causes issues in production:
1. **Don't run down migrations** — they don't exist
2. Fix forward with a new migration (e.g., `0054_revert_bad_column.up.sql`)
3. If data is corrupted, restore from RDS automated snapshot
4. Helm `--atomic` automatically rolls back the application deployment (not the schema)
