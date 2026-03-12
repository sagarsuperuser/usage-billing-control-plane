#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
environment="${ENVIRONMENT:-staging}"
run_go_tests="${RUN_GO_TESTS:-1}"
run_terraform_validate="${RUN_TERRAFORM_VALIDATE:-0}"
check_github="${CHECK_GITHUB:-0}"
github_repo="${GITHUB_REPOSITORY:-}"

if [[ "$environment" != "staging" && "$environment" != "prod" ]]; then
  echo "ENVIRONMENT must be one of: staging, prod" >&2
  exit 1
fi

failures=0
warnings=0

info() {
  printf '[info] %s\n' "$*"
}

pass() {
  printf '[pass] %s\n' "$*"
}

warn() {
  warnings=$((warnings + 1))
  printf '[warn] %s\n' "$*" >&2
}

fail() {
  failures=$((failures + 1))
  printf '[fail] %s\n' "$*" >&2
}

require_cmd() {
  local cmd="$1"
  if command -v "$cmd" >/dev/null 2>&1; then
    pass "command available: $cmd"
  else
    fail "missing required command: $cmd"
  fi
}

require_file() {
  local path="$1"
  if [[ -f "$path" ]]; then
    pass "file present: $path"
  else
    fail "missing required file: $path"
  fi
}

contains_name() {
  local needle="$1"
  shift
  local item
  for item in "$@"; do
    if [[ "$item" == "$needle" ]]; then
      return 0
    fi
  done
  return 1
}

split_lines_to_array() {
  local input="$1"
  local -n output_ref="$2"
  output_ref=()
  while IFS= read -r line; do
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    if [[ -n "$line" ]]; then
      output_ref+=("$line")
    fi
  done <<<"$input"
}

read_gh_names() {
  local scope="$1"
  local repo="$2"
  local env_name="${3:-}"
  local output
  if [[ "$scope" == "vars" ]]; then
    output="$(gh variable list --repo "$repo" 2>/dev/null || true)"
  elif [[ -n "$env_name" ]]; then
    output="$(gh secret list --repo "$repo" --env "$env_name" 2>/dev/null || true)"
  else
    output="$(gh secret list --repo "$repo" 2>/dev/null || true)"
  fi
  awk 'NR>1 {print $1}' <<<"$output"
}

run_cmd_check() {
  local label="$1"
  shift
  info "$label"
  if "$@"; then
    pass "$label"
  else
    fail "$label"
  fi
}

info "Running release preflight for ENVIRONMENT=$environment"

require_cmd git
require_cmd go
require_cmd docker
require_cmd terraform
require_cmd helm
require_cmd kubectl
require_cmd aws

tf_dir="$repo_root/infra/terraform/aws"
tf_var_file="$tf_dir/environments/${environment}.tfvars"
tf_backend_file="$tf_dir/backends/${environment}.hcl"
helm_chart="$repo_root/deploy/helm/lago-alpha"
helm_values="$helm_chart/environments/${environment}-values.yaml"

require_file "$tf_var_file"
require_file "$tf_backend_file"
require_file "$helm_values"
require_file "$repo_root/.github/workflows/release.yml"
require_file "$repo_root/.github/workflows/infra-deploy.yml"

run_cmd_check "terraform fmt -check" terraform fmt -check -recursive "$tf_dir"
run_cmd_check "helm lint" helm lint "$helm_chart"

rendered_file="/tmp/lago-alpha-${environment}.yaml"
run_cmd_check "helm template (${environment})" bash -lc "helm template lago-alpha '$helm_chart' -f '$helm_values' > '$rendered_file'"

if [[ -n "${TEST_DATABASE_URL:-}" ]]; then
  run_cmd_check "go test ./migrations -run TestRunnerAppliesMigrationsIdempotently -v" \
    bash -lc "cd '$repo_root' && TEST_DATABASE_URL='${TEST_DATABASE_URL}' go test ./migrations -run TestRunnerAppliesMigrationsIdempotently -v"
else
  warn "TEST_DATABASE_URL is not set, skipping migration integration test"
fi

run_cmd_check "verify governance metadata" \
  bash -lc "cd '$repo_root' && ALLOW_CODEOWNERS_PLACEHOLDERS=1 ./scripts/verify_codeowners.sh"

if [[ "$run_go_tests" == "1" ]]; then
  run_cmd_check "go test ./..." bash -lc "cd '$repo_root' && go test ./..."
else
  warn "RUN_GO_TESTS=0, skipping go test ./..."
fi

if [[ "$run_terraform_validate" == "1" ]]; then
  run_cmd_check "terraform init -backend=false" terraform -chdir="$tf_dir" init -backend=false
  run_cmd_check "terraform validate" terraform -chdir="$tf_dir" validate
else
  warn "RUN_TERRAFORM_VALIDATE=0, skipping terraform init/validate (provider download can be slow)"
fi

if [[ "$check_github" == "1" ]]; then
  require_cmd gh
  if [[ -z "$github_repo" ]]; then
    github_repo="$(gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null || true)"
  fi

  if [[ -z "$github_repo" ]]; then
    fail "CHECK_GITHUB=1 but GITHUB_REPOSITORY is not set and repo autodetect failed"
  else
    info "Checking GitHub Actions prerequisites in repo: $github_repo"

    declare -a required_vars=(
      "AWS_REGION"
      "ECR_API_REPOSITORY"
      "ECR_WEB_REPOSITORY"
      "EKS_CLUSTER_NAME_STAGING"
      "EKS_CLUSTER_NAME_PROD"
    )
    declare -a required_global_secrets=(
      "AWS_BUILD_ROLE_ARN"
      "AWS_DEPLOY_ROLE_ARN_STAGING"
      "AWS_DEPLOY_ROLE_ARN_PROD"
      "AWS_TERRAFORM_ROLE_ARN_STAGING"
      "AWS_TERRAFORM_ROLE_ARN_PROD"
      "TFVARS_STAGING_B64"
      "TF_BACKEND_STAGING_B64"
      "TFVARS_PROD_B64"
      "TF_BACKEND_PROD_B64"
    )

    vars_raw="$(read_gh_names vars "$github_repo")"
    secrets_raw="$(read_gh_names secrets "$github_repo")"
    env_staging_raw="$(read_gh_names secrets "$github_repo" staging)"
    env_prod_raw="$(read_gh_names secrets "$github_repo" production)"

    split_lines_to_array "$vars_raw" vars_list
    split_lines_to_array "$secrets_raw" secrets_list
    split_lines_to_array "$env_staging_raw" env_staging_secrets
    split_lines_to_array "$env_prod_raw" env_prod_secrets

    for name in "${required_vars[@]}"; do
      if contains_name "$name" "${vars_list[@]}"; then
        pass "github variable present: $name"
      else
        fail "missing github variable: $name"
      fi
    done

    for name in "${required_global_secrets[@]}"; do
      if contains_name "$name" "${secrets_list[@]}"; then
        pass "github secret present: $name"
      else
        fail "missing github secret: $name"
      fi
    done

    if [[ "${#env_staging_secrets[@]}" -eq 0 ]]; then
      warn "no environment-scoped secrets found for 'staging' (ok if all are repo-level)"
    fi
    if [[ "${#env_prod_secrets[@]}" -eq 0 ]]; then
      warn "no environment-scoped secrets found for 'production' (ok if all are repo-level)"
    fi
  fi
else
  warn "CHECK_GITHUB=0, skipping GitHub repository variable/secret checks"
fi

if [[ "$failures" -gt 0 ]]; then
  echo
  echo "Preflight failed: failures=$failures warnings=$warnings" >&2
  exit 1
fi

echo
echo "Preflight passed: failures=$failures warnings=$warnings"
echo "Next commands:"
echo "  make tf-plan-${environment}"
echo "  make tf-apply-${environment}"
echo "  make deploy-${environment} IMAGE_TAG=<sha> API_IMAGE_REPOSITORY=<ecr_api_repo> WEB_IMAGE_REPOSITORY=<ecr_web_repo>"
