ALTER TABLE tenants
  ADD COLUMN IF NOT EXISTS lago_api_key_secret_ref TEXT;
