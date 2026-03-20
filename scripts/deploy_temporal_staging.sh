#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
namespace="${TEMPORAL_NAMESPACE_K8S:-temporal}"
release_name="${TEMPORAL_RELEASE_NAME:-temporal}"
values_file="${TEMPORAL_VALUES_FILE:-$repo_root/deploy/temporal/environments/staging-values.yaml}"
chart_version="${TEMPORAL_CHART_VERSION:-0.73.2}"
chart_ref="${TEMPORAL_CHART_REF:-temporal/temporal}"
helm_repo_name="${TEMPORAL_HELM_REPO_NAME:-temporal}"
helm_repo_url="${TEMPORAL_HELM_REPO_URL:-https://go.temporal.io/helm-charts}"
db_instance_identifier="${TEMPORAL_DB_INSTANCE_IDENTIFIER:-lagoalphastagingdb}"
aws_region="${AWS_REGION:-us-east-1}"
sync_secrets="${TEMPORAL_SYNC_SECRETS:-1}"
sync_script="${TEMPORAL_SYNC_SCRIPT:-$repo_root/scripts/sync_temporal_staging_secrets.sh}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1" >&2; exit 1; }
}

require_file() {
  [[ -f "$1" ]] || { echo "required file not found: $1" >&2; exit 1; }
}

info() { printf '[info] %s\n' "$*"; }
pass() { printf '[pass] %s\n' "$*"; }

require_cmd helm
require_cmd kubectl
require_cmd aws
require_file "$values_file"
[[ "$sync_secrets" != "1" ]] || require_file "$sync_script"

kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null

if [[ "$sync_secrets" == "1" ]]; then
  info "ensuring Temporal SQL password secret is sourced from AWS Secrets Manager via ExternalSecret"
  TEMPORAL_NAMESPACE_K8S="$namespace" TEMPORAL_DB_INSTANCE_IDENTIFIER="$db_instance_identifier" AWS_REGION="$aws_region" "$sync_script"
fi

info "discovering RDS endpoint and master username for $db_instance_identifier"
read -r db_host db_user <<<"$(aws rds describe-db-instances \
  --db-instance-identifier "$db_instance_identifier" \
  --region "$aws_region" \
  --query 'DBInstances[0].[Endpoint.Address,MasterUsername]' \
  --output text)"

if [[ -z "$db_host" || -z "$db_user" ]]; then
  echo "failed to resolve RDS endpoint/username for $db_instance_identifier" >&2
  exit 1
fi

info "configuring Helm repo $helm_repo_name -> $helm_repo_url"
helm repo add "$helm_repo_name" "$helm_repo_url" >/dev/null 2>&1 || helm repo add "$helm_repo_name" "$helm_repo_url" --force-update >/dev/null
helm repo update "$helm_repo_name" >/dev/null

helm_args=(
  upgrade --install "$release_name" "$chart_ref"
  --namespace "$namespace"
  --create-namespace
  --atomic
  --timeout 20m
  --version "$chart_version"
  -f "$values_file"
  --set-string server.config.persistence.default.sql.host="$db_host"
  --set-string server.config.persistence.default.sql.user="$db_user"
  --set-string server.config.persistence.visibility.sql.host="$db_host"
  --set-string server.config.persistence.visibility.sql.user="$db_user"
)

info "deploying Temporal release=$release_name namespace=$namespace chart=$chart_ref version=$chart_version"
helm "${helm_args[@]}"

for deploy_name in temporal-frontend temporal-history temporal-matching temporal-worker temporal-admintools; do
  if kubectl -n "$namespace" get deploy "$deploy_name" >/dev/null 2>&1; then
    info "waiting for rollout $deploy_name"
    kubectl -n "$namespace" rollout status deployment/"$deploy_name" --timeout=10m
  fi
done

info "ensuring Temporal namespace default exists"
kubectl -n "$namespace" exec deploy/temporal-admintools -- sh -lc 'temporal operator namespace describe --namespace default >/dev/null 2>&1 || temporal operator namespace create --namespace default --retention 3d >/dev/null'

pass "Temporal staging deployment completed"
echo
printf 'TEMPORAL_ADDRESS=%s\n' "${release_name}-frontend.${namespace}.svc.cluster.local:7233"
