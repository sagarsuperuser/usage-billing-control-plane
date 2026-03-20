#!/usr/bin/env bash
set -euo pipefail

namespace="${TEMPORAL_NAMESPACE_K8S:-temporal}"
db_instance_identifier="${TEMPORAL_DB_INSTANCE_IDENTIFIER:-lagoalphastagingdb}"
aws_region="${AWS_REGION:-us-east-1}"
secret_name="${TEMPORAL_SQL_SECRET_NAME:-temporal-sql}"
cluster_secret_store_name="${TEMPORAL_CLUSTER_SECRET_STORE_NAME:-aws-secretsmanager}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1" >&2; exit 1; }
}

require_cmd aws
require_cmd kubectl
require_cmd jq

kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null

if ! kubectl get clustersecretstore "$cluster_secret_store_name" >/dev/null 2>&1; then
  echo "required ClusterSecretStore $cluster_secret_store_name not found" >&2
  exit 1
fi

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

cat <<EOF | kubectl apply -f - >/dev/null
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
    - secretKey: password
      remoteRef:
        key: ${secret_name_remote}
        property: password
EOF

for _ in $(seq 1 30); do
  if kubectl -n "$namespace" get secret "$secret_name" >/dev/null 2>&1; then
    echo "synced Temporal SQL password secret $secret_name into namespace $namespace via ExternalSecret"
    exit 0
  fi
  sleep 2
done

echo "timed out waiting for Temporal SQL password secret $secret_name in namespace $namespace" >&2
exit 1
