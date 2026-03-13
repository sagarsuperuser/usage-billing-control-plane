#!/usr/bin/env bash
set -euo pipefail

namespace="${LAGO_NAMESPACE:-lago}"
backing_secret_id="${LAGO_BACKING_SECRET_ID:-lago/staging/backing-services}"
app_secret_id="${LAGO_APP_SECRET_ID:-lago/staging/app-secrets}"
encryption_secret_id="${LAGO_ENCRYPTION_SECRET_ID:-lago/staging/encryption}"
cluster_secret_store_name="${LAGO_CLUSTER_SECRET_STORE_NAME:-aws-secretsmanager}"
credentials_secret_name="${LAGO_CREDENTIALS_SECRET_NAME:-lago-credentials}"
encryption_secret_name="${LAGO_ENCRYPTION_SECRET_NAME:-lago-encryption}"
app_secret_name="${LAGO_APP_SECRET_NAME:-lago-secrets}"
aws_region="${AWS_REGION:-us-east-1}"

echo_info() {
  printf '[info] %s\n' "$*"
}

echo_pass() {
  printf '[pass] %s\n' "$*"
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "missing required command: $cmd" >&2
    exit 1
  }
}

json_get() {
  local key="$1"
  jq -r --arg key "$key" '.[$key] // ""'
}

decode_base64() {
  if base64 --decode >/dev/null 2>&1 <<<"AQ=="; then
    base64 --decode
  else
    base64 -D
  fi
}

secret_exists_in_aws() {
  local secret_id="$1"
  aws secretsmanager describe-secret --secret-id "$secret_id" --region "$aws_region" >/dev/null 2>&1
}

read_k8s_secret_value() {
  local secret_name="$1"
  local key="$2"

  kubectl -n "$namespace" get secret "$secret_name" -o "jsonpath={.data.$key}" 2>/dev/null | decode_base64
}

ensure_aws_secret() {
  local secret_id="$1"
  local payload="$2"

  if secret_exists_in_aws "$secret_id"; then
    echo_info "aws secret $secret_id already exists"
    return 0
  fi

  echo_info "creating aws secret $secret_id"
  aws secretsmanager create-secret \
    --name "$secret_id" \
    --region "$aws_region" \
    --secret-string "$payload" >/dev/null
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

require_cmd aws
require_cmd kubectl
require_cmd openssl
require_cmd jq

if ! kubectl get clustersecretstore "$cluster_secret_store_name" >/dev/null 2>&1; then
  echo "required ClusterSecretStore $cluster_secret_store_name not found" >&2
  exit 1
fi

echo_info "ensuring namespace $namespace exists"
kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null

for _ in $(seq 1 30); do
  phase="$(kubectl get namespace "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
  if [[ "$phase" == "Active" ]]; then
    break
  fi
  sleep 2
done

phase="$(kubectl get namespace "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
if [[ "$phase" != "Active" ]]; then
  echo "namespace $namespace is not ready (phase=$phase)" >&2
  exit 1
fi

echo_info "reading Lago backing services secret metadata $backing_secret_id"
secret_json="$(aws secretsmanager get-secret-value \
  --secret-id "$backing_secret_id" \
  --region "$aws_region" \
  --query SecretString \
  --output text)"

database_url="$(printf '%s' "$secret_json" | json_get databaseUrl)"
redis_url="$(printf '%s' "$secret_json" | json_get redisUrl)"

if [[ -z "$database_url" || -z "$redis_url" ]]; then
  echo "secret $backing_secret_id must contain databaseUrl and redisUrl" >&2
  exit 1
fi

if kubectl -n "$namespace" get secret "$encryption_secret_name" >/dev/null 2>&1; then
  encryption_primary_key="$(read_k8s_secret_value "$encryption_secret_name" encryptionPrimaryKey)"
  encryption_deterministic_key="$(read_k8s_secret_value "$encryption_secret_name" encryptionDeterministicKey)"
  encryption_key_derivation_salt="$(read_k8s_secret_value "$encryption_secret_name" encryptionKeyDerivationSalt)"
else
  encryption_primary_key="$(openssl rand -hex 32)"
  encryption_deterministic_key="$(openssl rand -hex 32)"
  encryption_key_derivation_salt="$(openssl rand -hex 32)"
fi

encryption_payload="$(jq -nc \
  --arg encryptionPrimaryKey "$encryption_primary_key" \
  --arg encryptionDeterministicKey "$encryption_deterministic_key" \
  --arg encryptionKeyDerivationSalt "$encryption_key_derivation_salt" \
  '{
    encryptionPrimaryKey: $encryptionPrimaryKey,
    encryptionDeterministicKey: $encryptionDeterministicKey,
    encryptionKeyDerivationSalt: $encryptionKeyDerivationSalt
  }')"

if kubectl -n "$namespace" get secret "$app_secret_name" >/dev/null 2>&1; then
  secret_key_base="$(read_k8s_secret_value "$app_secret_name" secretKeyBase)"
  rsa_private_key="$(read_k8s_secret_value "$app_secret_name" rsaPrivateKey)"
else
  secret_key_base="$(openssl rand -hex 64)"
  rsa_private_key="$(openssl genrsa 2048 2>/dev/null | openssl base64 -A)"
fi

app_payload="$(jq -nc \
  --arg secretKeyBase "$secret_key_base" \
  --arg rsaPrivateKey "$rsa_private_key" \
  '{
    secretKeyBase: $secretKeyBase,
    rsaPrivateKey: $rsaPrivateKey
  }')"

ensure_aws_secret "$encryption_secret_id" "$encryption_payload"
ensure_aws_secret "$app_secret_id" "$app_payload"

echo_info "applying ExternalSecret resources in namespace $namespace"
cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: ${credentials_secret_name}
  namespace: ${namespace}
spec:
  refreshInterval: 1m
  secretStoreRef:
    kind: ClusterSecretStore
    name: ${cluster_secret_store_name}
  target:
    name: ${credentials_secret_name}
    creationPolicy: Owner
  dataFrom:
    - extract:
        key: ${backing_secret_id}
---
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: ${encryption_secret_name}
  namespace: ${namespace}
spec:
  refreshInterval: 1m
  secretStoreRef:
    kind: ClusterSecretStore
    name: ${cluster_secret_store_name}
  target:
    name: ${encryption_secret_name}
    creationPolicy: Owner
  dataFrom:
    - extract:
        key: ${encryption_secret_id}
---
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: ${app_secret_name}
  namespace: ${namespace}
spec:
  refreshInterval: 1m
  secretStoreRef:
    kind: ClusterSecretStore
    name: ${cluster_secret_store_name}
  target:
    name: ${app_secret_name}
    creationPolicy: Owner
  dataFrom:
    - extract:
        key: ${app_secret_id}
EOF

wait_for_k8s_secret "$credentials_secret_name"
wait_for_k8s_secret "$encryption_secret_name"
wait_for_k8s_secret "$app_secret_name"

echo_pass "Lago Kubernetes secrets are synced via ExternalSecret"
echo "  namespace: $namespace"
echo "  credentials secret: $credentials_secret_name"
echo "  encryption secret: $encryption_secret_name"
echo "  application secret: $app_secret_name"
echo "  backing aws secret: $backing_secret_id"
echo "  encryption aws secret: $encryption_secret_id"
echo "  app aws secret: $app_secret_id"
