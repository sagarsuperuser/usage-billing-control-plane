ALTER TABLE rating_rule_versions
  ADD COLUMN IF NOT EXISTS rule_key TEXT;

WITH normalized AS (
  SELECT
    r.id,
    COALESCE(
      NULLIF(
        regexp_replace(
          lower(trim(COALESCE(r.name, ''))),
          '[^a-z0-9_-]+',
          '_',
          'g'
        ),
        ''
      ),
      'rule_' || substring(md5(r.id) for 8)
    ) AS normalized_key,
    row_number() OVER (
      PARTITION BY r.tenant_id, r.version,
        COALESCE(
          NULLIF(
            regexp_replace(
              lower(trim(COALESCE(r.name, ''))),
              '[^a-z0-9_-]+',
              '_',
              'g'
            ),
            ''
          ),
          'rule_' || substring(md5(r.id) for 8)
        )
      ORDER BY r.created_at ASC, r.id ASC
    ) AS rn
  FROM rating_rule_versions r
)
UPDATE rating_rule_versions r
SET rule_key = CASE
  WHEN n.rn = 1 THEN n.normalized_key
  ELSE n.normalized_key || '_' || substring(md5(r.id) for 6)
END
FROM normalized n
WHERE r.id = n.id
  AND (r.rule_key IS NULL OR btrim(r.rule_key) = '');

UPDATE rating_rule_versions
SET rule_key = regexp_replace(lower(trim(rule_key)), '[^a-z0-9_-]+', '_', 'g')
WHERE rule_key IS NOT NULL;

UPDATE rating_rule_versions
SET rule_key = 'rule_' || substring(md5(id) for 8)
WHERE rule_key IS NULL OR btrim(rule_key) = '';

ALTER TABLE rating_rule_versions
  ALTER COLUMN rule_key SET NOT NULL;

ALTER TABLE rating_rule_versions
  ADD COLUMN IF NOT EXISTS lifecycle_state TEXT DEFAULT 'active';

UPDATE rating_rule_versions
SET lifecycle_state = 'active'
WHERE lifecycle_state IS NULL OR btrim(lifecycle_state) = '';

ALTER TABLE rating_rule_versions
  ALTER COLUMN lifecycle_state SET DEFAULT 'active',
  ALTER COLUMN lifecycle_state SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_rating_rule_versions_rule_key_format'
  ) THEN
    ALTER TABLE rating_rule_versions
      ADD CONSTRAINT chk_rating_rule_versions_rule_key_format
      CHECK (rule_key ~ '^[a-z0-9][a-z0-9_-]{0,63}$');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'chk_rating_rule_versions_lifecycle_state'
  ) THEN
    ALTER TABLE rating_rule_versions
      ADD CONSTRAINT chk_rating_rule_versions_lifecycle_state
      CHECK (lifecycle_state IN ('draft', 'active', 'archived'));
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rating_rule_versions_tenant_rule_key_version
  ON rating_rule_versions (tenant_id, rule_key, version);

WITH ranked AS (
  SELECT
    id,
    row_number() OVER (
      PARTITION BY tenant_id, rule_key
      ORDER BY version DESC, created_at DESC, id DESC
    ) AS rn
  FROM rating_rule_versions
  WHERE lifecycle_state = 'active'
)
UPDATE rating_rule_versions r
SET lifecycle_state = 'archived'
FROM ranked x
WHERE r.id = x.id
  AND x.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rating_rule_versions_tenant_rule_key_active
  ON rating_rule_versions (tenant_id, rule_key)
  WHERE lifecycle_state = 'active';

CREATE INDEX IF NOT EXISTS idx_rating_rule_versions_tenant_rule_key_version
  ON rating_rule_versions (tenant_id, rule_key, version DESC, created_at DESC);

