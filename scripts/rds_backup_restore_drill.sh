#!/usr/bin/env bash
set -euo pipefail

required_cmds=(aws jq)
for cmd in "${required_cmds[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
done

require_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    echo "missing required environment variable: $key" >&2
    exit 1
  fi
}

trim() {
  echo "$1" | xargs
}

bool_env() {
  local raw
  raw="$(echo "${1:-}" | tr '[:upper:]' '[:lower:]' | xargs)"
  case "$raw" in
    1|true|yes|y) echo "1" ;;
    0|false|no|n|"") echo "0" ;;
    *)
      echo "invalid boolean value: $1" >&2
      exit 1
      ;;
  esac
}

run_cmd() {
  if [[ "$PLAN_ONLY" == "1" ]]; then
    printf '[plan] '
    printf '%q ' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

ENVIRONMENT="$(trim "${ENVIRONMENT:-staging}")"
AWS_REGION="$(trim "${AWS_REGION:-}")"
RDS_INSTANCE_ID="$(trim "${RDS_INSTANCE_ID:-}")"
DB_SUBNET_GROUP="$(trim "${DB_SUBNET_GROUP:-}")"
VPC_SG_IDS_RAW="$(trim "${VPC_SG_IDS:-${VPC_SG_ID:-}}")"
RESTORE_DB_INSTANCE_CLASS="$(trim "${RESTORE_DB_INSTANCE_CLASS:-}")"
PUBLICLY_ACCESSIBLE="$(bool_env "${PUBLICLY_ACCESSIBLE:-0}")"
DELETE_RESTORE_ON_SUCCESS="$(bool_env "${DELETE_RESTORE_ON_SUCCESS:-0}")"
DELETE_RESTORE_SKIP_FINAL_SNAPSHOT="$(bool_env "${DELETE_RESTORE_SKIP_FINAL_SNAPSHOT:-1}")"
WAIT_FOR_DELETE="$(bool_env "${WAIT_FOR_DELETE:-0}")"
PLAN_ONLY="$(bool_env "${PLAN_ONLY:-0}")"
CONFIRM_BACKUP_RESTORE="$(trim "${CONFIRM_BACKUP_RESTORE:-}")"
ALLOW_PROD_DRILL="$(bool_env "${ALLOW_PROD_DRILL:-0}")"

require_env AWS_REGION
require_env RDS_INSTANCE_ID

if [[ "$ENVIRONMENT" == "prod" && "$ALLOW_PROD_DRILL" != "1" ]]; then
  echo "refusing to run in prod without ALLOW_PROD_DRILL=1" >&2
  exit 1
fi

if [[ "$CONFIRM_BACKUP_RESTORE" != "YES_I_UNDERSTAND" ]]; then
  echo "set CONFIRM_BACKUP_RESTORE=YES_I_UNDERSTAND to execute backup/restore drill" >&2
  exit 1
fi

timestamp="$(date +%Y%m%d%H%M%S)"
SNAPSHOT_ID="$(trim "${SNAPSHOT_ID:-lago-alpha-${ENVIRONMENT}-drill-snap-${timestamp}}")"
RESTORE_INSTANCE_ID="$(trim "${RESTORE_INSTANCE_ID:-lago-alpha-${ENVIRONMENT}-drill-restore-${timestamp}}")"
RESTORE_FINAL_SNAPSHOT_ID="$(trim "${RESTORE_FINAL_SNAPSHOT_ID:-${RESTORE_INSTANCE_ID}-final-${timestamp}}")"

if [[ ${#SNAPSHOT_ID} -gt 255 || ${#RESTORE_INSTANCE_ID} -gt 63 ]]; then
  echo "identifier too long (snapshot <=255, restore instance <=63)" >&2
  exit 1
fi

echo "[info] validating source db instance: $RDS_INSTANCE_ID"
run_cmd aws rds describe-db-instances \
  --region "$AWS_REGION" \
  --db-instance-identifier "$RDS_INSTANCE_ID" >/dev/null
echo "[pass] source db instance found"

echo "[info] creating snapshot: $SNAPSHOT_ID"
run_cmd aws rds create-db-snapshot \
  --region "$AWS_REGION" \
  --db-instance-identifier "$RDS_INSTANCE_ID" \
  --db-snapshot-identifier "$SNAPSHOT_ID" >/dev/null

echo "[info] waiting for snapshot availability"
run_cmd aws rds wait db-snapshot-available \
  --region "$AWS_REGION" \
  --db-snapshot-identifier "$SNAPSHOT_ID"
echo "[pass] snapshot is available"

restore_cmd=(
  aws rds restore-db-instance-from-db-snapshot
  --region "$AWS_REGION"
  --db-instance-identifier "$RESTORE_INSTANCE_ID"
  --db-snapshot-identifier "$SNAPSHOT_ID"
)
if [[ -n "$DB_SUBNET_GROUP" ]]; then
  restore_cmd+=(--db-subnet-group-name "$DB_SUBNET_GROUP")
fi
if [[ -n "$RESTORE_DB_INSTANCE_CLASS" ]]; then
  restore_cmd+=(--db-instance-class "$RESTORE_DB_INSTANCE_CLASS")
fi
if [[ "$PUBLICLY_ACCESSIBLE" == "1" ]]; then
  restore_cmd+=(--publicly-accessible)
else
  restore_cmd+=(--no-publicly-accessible)
fi
if [[ -n "$VPC_SG_IDS_RAW" ]]; then
  IFS=',' read -r -a sg_ids <<<"$VPC_SG_IDS_RAW"
  cleaned_sg_ids=()
  for id in "${sg_ids[@]}"; do
    id="$(trim "$id")"
    if [[ -n "$id" ]]; then
      cleaned_sg_ids+=("$id")
    fi
  done
  if [[ ${#cleaned_sg_ids[@]} -gt 0 ]]; then
    restore_cmd+=(--vpc-security-group-ids)
    restore_cmd+=("${cleaned_sg_ids[@]}")
  fi
fi

echo "[info] restoring db instance: $RESTORE_INSTANCE_ID"
run_cmd "${restore_cmd[@]}" >/dev/null

echo "[info] waiting for restored db instance availability"
run_cmd aws rds wait db-instance-available \
  --region "$AWS_REGION" \
  --db-instance-identifier "$RESTORE_INSTANCE_ID"

if [[ "$PLAN_ONLY" == "1" ]]; then
  restored_endpoint="<plan-only>"
  restored_status="<plan-only>"
else
  restored_json="$(
    run_cmd aws rds describe-db-instances \
      --region "$AWS_REGION" \
      --db-instance-identifier "$RESTORE_INSTANCE_ID" \
      --query 'DBInstances[0]' \
      --output json
  )"
  restored_endpoint="$(jq -r '.Endpoint.Address // empty' <<<"$restored_json")"
  restored_status="$(jq -r '.DBInstanceStatus // empty' <<<"$restored_json")"
fi
echo "[pass] restored instance is available: status=$restored_status endpoint=${restored_endpoint:-<pending>}"

deleted_restore_instance=0
if [[ "$DELETE_RESTORE_ON_SUCCESS" == "1" ]]; then
  echo "[info] deleting restored instance: $RESTORE_INSTANCE_ID"
  delete_cmd=(
    aws rds delete-db-instance
    --region "$AWS_REGION"
    --db-instance-identifier "$RESTORE_INSTANCE_ID"
    --delete-automated-backups
  )
  if [[ "$DELETE_RESTORE_SKIP_FINAL_SNAPSHOT" == "1" ]]; then
    delete_cmd+=(--skip-final-snapshot)
  else
    delete_cmd+=(--final-db-snapshot-identifier "$RESTORE_FINAL_SNAPSHOT_ID")
  fi
  run_cmd "${delete_cmd[@]}" >/dev/null
  deleted_restore_instance=1

  if [[ "$WAIT_FOR_DELETE" == "1" ]]; then
    echo "[info] waiting for restored instance deletion"
    run_cmd aws rds wait db-instance-deleted \
      --region "$AWS_REGION" \
      --db-instance-identifier "$RESTORE_INSTANCE_ID"
    echo "[pass] restored instance deleted"
  fi
fi

jq -n \
  --arg environment "$ENVIRONMENT" \
  --arg aws_region "$AWS_REGION" \
  --arg source_instance_id "$RDS_INSTANCE_ID" \
  --arg snapshot_id "$SNAPSHOT_ID" \
  --arg restore_instance_id "$RESTORE_INSTANCE_ID" \
  --arg restore_endpoint "$restored_endpoint" \
  --arg restore_status "$restored_status" \
  --argjson deleted_restore_instance "$deleted_restore_instance" \
  --argjson plan_only "$PLAN_ONLY" \
  '{
    environment: $environment,
    aws_region: $aws_region,
    source_instance_id: $source_instance_id,
    snapshot_id: $snapshot_id,
    restore_instance_id: $restore_instance_id,
    restore_endpoint: $restore_endpoint,
    restore_status: $restore_status,
    deleted_restore_instance: ($deleted_restore_instance == 1),
    plan_only: ($plan_only == 1)
  }'
