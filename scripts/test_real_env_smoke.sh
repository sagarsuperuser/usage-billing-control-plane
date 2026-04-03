#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

integration_test_pattern="${INTEGRATION_TEST_PATTERN:-TestEndToEndPreviewReplayReconciliation|TestRatingRuleGovernanceLifecycle}"
integration_run_migrations_test="${INTEGRATION_RUN_MIGRATIONS_TEST:-1}"

INTEGRATION_TEST_PATTERN="$integration_test_pattern" \
INTEGRATION_RUN_MIGRATIONS_TEST="$integration_run_migrations_test" \
RUN_LARGE_REPLAY_DATASET=0 \
bash "$repo_root/scripts/test_integration.sh"
