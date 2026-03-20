#!/usr/bin/env bash
set -euo pipefail

namespace="${TEMPORAL_NAMESPACE_K8S:-temporal}"
db_instance_identifier="${TEMPORAL_DB_INSTANCE_IDENTIFIER:-lagoalphastagingdb}"
aws_region="${AWS_REGION:-us-east-1}"
cluster_secret_store_name="${TEMPORAL_CLUSTER_SECRET_STORE_NAME:-aws-secretsmanager}"
sql_secret_name="${TEMPORAL_SQL_SECRET_NAME:-temporal-sql}"
sql_secret_key="${TEMPORAL_SQL_SECRET_KEY:-password}"
sql_secret_remote_property="${TEMPORAL_SQL_SECRET_REMOTE_PROPERTY:-password}"

info() {
  printf '[info] %s\n' "$*"
}

pass() {
  printf '[pass] %s\n' "$*"
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "missing required command: $cmd" >&2
    exit 1
  }
}

wait_for_k8s_secret() {
  local secret_name="$1"

  for _ in $(seq 1 30); do
    if kubectl -n "$namespace" get secret "$secret_name" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  echo "timed out waiting for kubernetes secret $secret_name in namespace $namespace" >&2
  exit 1
}

apply_external_secret() {
  local secret_name="$1"
  local secret_key="$2"
  local remote_secret_name="$3"
  local remote_property="$4"

  cat <<MANIFEST | kubectl apply -f - >/dev/null
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: ${secret_name}
  namespace: ${namespace}
spec:
  refreshInterval: 1m
  secretStoreRef:
    kind: ClusterSecretStore
    name: ${cluster_secret_store_name}
  target:
    name: ${secret_name}
    creationPolicy: Owner
  data:
    - secretKey: ${secret_key}
      remoteRef:
        key: ${remote_secret_name}
        property: ${remote_property}
MANIFEST
}

resolve_rds_master_secret_name() {
  local secret_arn
  local secret_name_remote

  secret_arn="$(aws rds describe-db-instances \
    --db-instance-identifier "$db_instance_identifier" \
    --region "$aws_region" \
    --query 'DBInstances[0].MasterUserSecret.SecretArn' \
    --output text)"

  if [[ -z "$secret_arn" || "$secret_arn" == "None" ]]; then
    echo "failed to discover RDS master secret for $db_instance_identifier" >&2
    exit 1
  fi

  secret_name_remote="$(aws secretsmanager describe-secret \
    --secret-id "$secret_arn" \
    --region "$aws_region" \
    --query 'Name' \
    --output text)"

  if [[ -z "$secret_name_remote" || "$secret_name_remote" == "None" ]]; then
    echo "failed to resolve Secrets Manager name for $secret_arn" >&2
    exit 1
  fi

  printf '%s' "$secret_name_remote"
}

require_cmd aws
require_cmd kubectl

info "ensuring namespace $namespace exists"
kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null

if ! kubectl get clustersecretstore "$cluster_secret_store_name" >/dev/null 2>&1; then
  echo "required ClusterSecretStore $cluster_secret_store_name not found" >&2
  exit 1
fi

info "discovering AWS Secrets Manager source for RDS instance $db_instance_identifier"
rds_master_secret_name="$(resolve_rds_master_secret_name)"

info "applying Temporal ExternalSecret resources in namespace $namespace"
apply_external_secret "$sql_secret_name" "$sql_secret_key" "$rds_master_secret_name" "$sql_secret_remote_property"

wait_for_k8s_secret "$sql_secret_name"

pass "Temporal Kubernetes secrets are prepared"
echo "  namespace: $namespace"
echo "  sql secret: $sql_secret_name"
echo "  aws secret source: $rds_master_secret_name"
