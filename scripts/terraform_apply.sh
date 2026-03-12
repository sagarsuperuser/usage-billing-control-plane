#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tf_dir="${TF_DIR:-$repo_root/infra/terraform/aws}"
environment="${ENVIRONMENT:-}"
plan_file="${PLAN_FILE:-$repo_root/infra/terraform/aws/tfplan-${ENVIRONMENT:-unknown}.bin}"
skip_backend_init="${SKIP_BACKEND_INIT:-0}"

if [[ "$environment" != "staging" && "$environment" != "prod" ]]; then
  echo "ENVIRONMENT must be one of: staging, prod" >&2
  exit 1
fi

backend_file="$tf_dir/backends/${environment}.hcl"

if [[ ! -f "$plan_file" ]]; then
  echo "plan file not found: $plan_file" >&2
  echo "run scripts/terraform_plan.sh first" >&2
  exit 1
fi

if [[ "$skip_backend_init" == "1" ]]; then
  terraform -chdir="$tf_dir" init -backend=false
else
  if [[ ! -f "$backend_file" ]]; then
    echo "missing backend config: $backend_file" >&2
    echo "copy from ${backend_file}.example and fill remote state values" >&2
    exit 1
  fi
  terraform -chdir="$tf_dir" init -reconfigure -backend-config="$backend_file"
fi

terraform -chdir="$tf_dir" apply "$plan_file"
