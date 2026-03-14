#!/usr/bin/env bash
set -euo pipefail

cat <<'TXT'
Lago staging bootstrap checklist

Automated:
- namespace and Helm deployment
- rollout verification
- basic API reachability verification

Manual first-time steps:
1. Provision Lago Postgres.
2. Provision Lago Redis.
3. Provision object storage for Lago uploads.
4. Fill deploy/lago/environments/staging-values.yaml.
5. Generate Lago API key.
6. Configure Stripe test mode in Lago.
7. Create first staging customer/org for payment E2E.
8. Feed LAGO_API_URL and LAGO_API_KEY into alpha runtime secret.
TXT
