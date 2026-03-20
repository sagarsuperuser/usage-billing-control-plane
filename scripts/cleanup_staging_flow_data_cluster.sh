#!/usr/bin/env bash
set -euo pipefail

namespace="${NAMESPACE:-lago-alpha}"
release_name="${RELEASE_NAME:-lago-alpha}"
api_deployment="${API_DEPLOYMENT_NAME:-${release_name}-lago-alpha-api}"
output="${OUTPUT:-json}"
environment="${ENVIRONMENT:-staging}"
job_timeout="${JOB_TIMEOUT:-180s}"
ttl_seconds="${TTL_SECONDS_AFTER_FINISHED:-600}"
cleanup_job="${CLEANUP_JOB:-1}"
apply="${APPLY:-0}"
include_replay="${INCLUDE_REPLAY_FIXTURES:-1}"
include_payment="${INCLUDE_PAYMENT_FIXTURES:-1}"
include_live_browser="${INCLUDE_LIVE_BROWSER_FIXTURES:-1}"
job_name_input="${JOB_NAME:-cleanup-staging-flow-data-$(date +%Y%m%d%H%M%S)}"
job_name="$(printf '%s' "$job_name_input" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9-' '-' | sed 's/^-//; s/-$//' | cut -c1-63)"

if [[ -z "$job_name" ]]; then
  echo "JOB_NAME resolved to empty value" >&2
  exit 1
fi

if [[ "$apply" == "1" && "${CONFIRM_STAGING_FLOW_CLEANUP:-}" != "YES_I_UNDERSTAND" ]]; then
  echo "set CONFIRM_STAGING_FLOW_CLEANUP=YES_I_UNDERSTAND to apply staging cleanup" >&2
  exit 1
fi

image="$(kubectl -n "$namespace" get deploy "$api_deployment" -o jsonpath='{.spec.template.spec.containers[0].image}')"
service_account="$(kubectl -n "$namespace" get deploy "$api_deployment" -o jsonpath='{.spec.template.spec.serviceAccountName}')"
config_map_ref="$(kubectl -n "$namespace" get deploy "$api_deployment" -o jsonpath='{.spec.template.spec.containers[0].envFrom[?(@.configMapRef)].configMapRef.name}')"
secret_ref="$(kubectl -n "$namespace" get deploy "$api_deployment" -o jsonpath='{.spec.template.spec.containers[0].envFrom[?(@.secretRef)].secretRef.name}')"

if [[ -z "$image" || -z "$service_account" || -z "$config_map_ref" || -z "$secret_ref" ]]; then
  echo "failed to derive runtime wiring from deployment $api_deployment" >&2
  exit 1
fi

args=(
  "cleanup-staging-fixtures"
  "-environment" "$environment"
  "-output" "$output"
)

if [[ "$apply" == "1" ]]; then
  args+=( "-apply" )
fi
if [[ "$include_replay" != "1" ]]; then
  args+=( "-include-replay-fixtures=false" )
fi
if [[ "$include_payment" != "1" ]]; then
  args+=( "-include-payment-fixtures=false" )
fi
if [[ "$include_live_browser" != "1" ]]; then
  args+=( "-include-live-browser-fixtures=false" )
fi

manifest_file="$(mktemp)"
trap 'rm -f "$manifest_file"' EXIT

cat >"$manifest_file" <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job_name}
  namespace: ${namespace}
spec:
  ttlSecondsAfterFinished: ${ttl_seconds}
  backoffLimit: 0
  template:
    metadata:
      labels:
        app.kubernetes.io/name: lago-alpha
        app.kubernetes.io/instance: ${release_name}
        app.kubernetes.io/component: admin-cleanup
    spec:
      restartPolicy: Never
      serviceAccountName: ${service_account}
      containers:
        - name: cleanup-staging-flow-data
          image: ${image}
          imagePullPolicy: IfNotPresent
          command: ["/app/admin"]
          args:
EOF

for arg in "${args[@]}"; do
  escaped_arg="${arg//\\/\\\\}"
  escaped_arg="${escaped_arg//\"/\\\"}"
  printf '            - "%s"\n' "$escaped_arg" >>"$manifest_file"
done

cat >>"$manifest_file" <<EOF
          envFrom:
            - configMapRef:
                name: ${config_map_ref}
            - secretRef:
                name: ${secret_ref}
EOF

kubectl apply -f "$manifest_file" >/dev/null

if ! kubectl -n "$namespace" wait --for=condition=complete "job/${job_name}" --timeout="$job_timeout" >/dev/null; then
  kubectl -n "$namespace" describe "job/${job_name}" >&2 || true
  kubectl -n "$namespace" logs "job/${job_name}" >&2 || true
  exit 1
fi

kubectl -n "$namespace" logs "job/${job_name}"

if [[ "$cleanup_job" == "1" ]]; then
  kubectl -n "$namespace" delete "job/${job_name}" --ignore-not-found >/dev/null
fi
