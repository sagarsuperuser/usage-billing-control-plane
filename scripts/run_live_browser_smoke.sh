#!/usr/bin/env bash
set -euo pipefail

required_envs=(
  PLAYWRIGHT_LIVE_BASE_URL
  PLAYWRIGHT_LIVE_PLATFORM_EMAIL
  PLAYWRIGHT_LIVE_PLATFORM_PASSWORD
  PLAYWRIGHT_LIVE_WRITER_EMAIL
  PLAYWRIGHT_LIVE_WRITER_PASSWORD
  PLAYWRIGHT_LIVE_READER_EMAIL
  PLAYWRIGHT_LIVE_READER_PASSWORD
)

for key in "${required_envs[@]}"; do
  if [[ -z "${!key:-}" ]]; then
    echo "missing required environment variable: $key" >&2
    exit 1
  fi
done

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$repo_root/web"
npx -y pnpm@10.30.0 exec playwright install --with-deps chromium
npx -y pnpm@10.30.0 exec playwright test \
  tests/e2e/control-plane-overview-live.spec.ts \
  tests/e2e/payment-operations-live.spec.ts \
  tests/e2e/invoice-explainability-live.spec.ts \
  tests/e2e/replay-operations-live.spec.ts \
  --workers=1
