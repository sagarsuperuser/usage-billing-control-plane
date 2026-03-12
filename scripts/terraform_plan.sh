#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tf_dir="${TF_DIR:-$repo_root/infra/terraform/aws}"
environment="${ENVIRONMENT:-}"
plan_out="${PLAN_OUT:-$repo_root/infra/terraform/aws/tfplan-${ENVIRONMENT:-unknown}.bin}"
skip_backend_init="${SKIP_BACKEND_INIT:-0}"
extra_plan_args="${EXTRA_PLAN_ARGS:-}"

if [[ "$environment" != "staging" && "$environment" != "prod" ]]; then
  echo "ENVIRONMENT must be one of: staging, prod" >&2
  exit 1
fi

backend_file="$tf_dir/backends/${environment}.hcl"
var_file="$tf_dir/environments/${environment}.tfvars"

if [[ ! -f "$var_file" ]]; then
  echo "missing environment var file: $var_file" >&2
  echo "copy from ${var_file}.example and fill required values" >&2
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

terraform -chdir="$tf_dir" plan -var-file="$var_file" -out="$plan_out" ${extra_plan_args}
terraform -chdir="$tf_dir" show "$plan_out"
