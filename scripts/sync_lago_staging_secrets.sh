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
lago_db_instance_identifier="${LAGO_DB_INSTANCE_IDENTIFIER:-lagostagingdb}"
lago_db_sslmode="${LAGO_DB_SSLMODE:-require}"
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

echo_info "reading Lago RDS metadata from $lago_db_instance_identifier"
db_json="$(aws rds describe-db-instances \
  --db-instance-identifier "$lago_db_instance_identifier" \
  --region "$aws_region" \
  --query 'DBInstances[0].{endpoint:Endpoint.Address,port:Endpoint.Port,dbName:DBName,masterSecretArn:MasterUserSecret.SecretArn,masterUsername:MasterUsername}' \
  --output json)"

db_endpoint="$(printf '%s' "$db_json" | jq -r '.endpoint // ""')"
db_port="$(printf '%s' "$db_json" | jq -r '.port // ""')"
db_name="$(printf '%s' "$db_json" | jq -r '.dbName // ""')"
db_master_secret_arn="$(printf '%s' "$db_json" | jq -r '.masterSecretArn // ""')"
db_master_username="$(printf '%s' "$db_json" | jq -r '.masterUsername // ""')"

if [[ -z "$db_endpoint" || -z "$db_port" || -z "$db_name" || -z "$db_master_secret_arn" ]]; then
  echo "rds instance $lago_db_instance_identifier is missing endpoint/port/dbName/master secret metadata" >&2
  exit 1
fi

db_master_secret_json="$(aws secretsmanager get-secret-value \
  --secret-id "$db_master_secret_arn" \
  --region "$aws_region" \
  --query SecretString \
  --output text)"

db_username="$(printf '%s' "$db_master_secret_json" | jq -r '.username // empty')"
db_password="$(printf '%s' "$db_master_secret_json" | jq -r '.password // empty')"

if [[ -z "$db_username" ]]; then
  db_username="$db_master_username"
fi

if [[ -z "$db_username" || -z "$db_password" ]]; then
  echo "rds master secret $db_master_secret_arn is missing username/password" >&2
  exit 1
fi

encoded_db_password="$(jq -nr --arg value "$db_password" '$value|@uri')"
reconciled_database_url="postgresql://${db_username}:${encoded_db_password}@${db_endpoint}:${db_port}/${db_name}?sslmode=${lago_db_sslmode}"

existing_database_url="$(printf '%s' "$secret_json" | json_get databaseUrl)"
redis_url="$(printf '%s' "$secret_json" | json_get redisUrl)"
redis_cache_url="$(printf '%s' "$secret_json" | json_get redisCacheUrl)"

if [[ -z "$redis_url" ]]; then
  echo "secret $backing_secret_id must contain redisUrl" >&2
  exit 1
fi

updated_secret_json="$(printf '%s' "$secret_json" | jq -c --arg databaseUrl "$reconciled_database_url" '. + {databaseUrl: $databaseUrl}')"

if [[ "$existing_database_url" != "$reconciled_database_url" ]]; then
  echo_info "updating aws secret $backing_secret_id to reconcile databaseUrl from live RDS credentials"
fi

if [[ -z "$redis_cache_url" ]]; then
  redis_cache_url="$redis_url"
  updated_secret_json="$(printf '%s' "$updated_secret_json" | jq -c --arg redisCacheUrl "$redis_cache_url" '. + {redisCacheUrl: $redisCacheUrl}')"
  echo_info "updating aws secret $backing_secret_id to include redisCacheUrl"
fi

if [[ "$updated_secret_json" != "$secret_json" ]]; then
  aws secretsmanager put-secret-value \
    --secret-id "$backing_secret_id" \
    --region "$aws_region" \
    --secret-string "$updated_secret_json" >/dev/null
  secret_json="$updated_secret_json"
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
  admin_api_key="$(read_k8s_secret_value "$app_secret_name" adminApiKey)"
else
  secret_key_base="$(openssl rand -hex 64)"
  rsa_private_key="$(openssl genrsa 2048 2>/dev/null | openssl base64 -A)"
  admin_api_key="$(openssl rand -hex 32)"
fi

if [[ -z "$admin_api_key" ]]; then
  admin_api_key="$(openssl rand -hex 32)"
fi

app_payload="$(jq -nc \
  --arg secretKeyBase "$secret_key_base" \
  --arg rsaPrivateKey "$rsa_private_key" \
  --arg adminApiKey "$admin_api_key" \
  '{
    secretKeyBase: $secretKeyBase,
    rsaPrivateKey: $rsaPrivateKey,
    adminApiKey: $adminApiKey
  }')"

ensure_aws_secret "$encryption_secret_id" "$encryption_payload"
ensure_aws_secret "$app_secret_id" "$app_payload"

echo_info "applying ExternalSecret resources in namespace $namespace"
cat <<EOK | kubectl apply -f - >/dev/null
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
EOK

wait_for_k8s_secret "$credentials_secret_name"
wait_for_k8s_secret "$encryption_secret_name"

if kubectl -n "$namespace" get externalsecret "$app_secret_name" >/dev/null 2>&1; then
  echo_info "removing ExternalSecret $app_secret_name to avoid Helm hook ownership conflicts"
  kubectl -n "$namespace" delete externalsecret "$app_secret_name" >/dev/null
  kubectl -n "$namespace" wait --for=delete "externalsecret/$app_secret_name" --timeout=60s >/dev/null
fi

echo_info "upserting Kubernetes secret $app_secret_name from AWS seed data"
kubectl -n "$namespace" create secret generic "$app_secret_name" \
  --from-literal=secretKeyBase="$secret_key_base" \
  --from-literal=rsaPrivateKey="$rsa_private_key" \
  --from-literal=adminApiKey="$admin_api_key" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

wait_for_k8s_secret "$app_secret_name"

echo_pass "Lago Kubernetes secrets are prepared"
echo "  namespace: $namespace"
echo "  credentials secret: $credentials_secret_name"
echo "  encryption secret: $encryption_secret_name"
echo "  application secret: $app_secret_name (seeded from AWS; chart-managed thereafter)"
echo "  backing aws secret: $backing_secret_id"
echo "  encryption aws secret: $encryption_secret_id"
echo "  app aws secret: $app_secret_id"
echo "  reconciled db instance: $lago_db_instance_identifier"
