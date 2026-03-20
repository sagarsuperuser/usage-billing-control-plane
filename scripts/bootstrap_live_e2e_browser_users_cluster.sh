#!/usr/bin/env bash
set -euo pipefail

namespace="${NAMESPACE:-lago-alpha}"
release_name="${RELEASE_NAME:-lago-alpha}"
api_deployment="${API_DEPLOYMENT_NAME:-${release_name}-lago-alpha-api}"
tenant_id="${TARGET_TENANT_ID:-default}"
tenant_name="${TARGET_TENANT_NAME:-}"
output="${OUTPUT:-shell}"
job_timeout="${JOB_TIMEOUT:-180s}"
ttl_seconds="${TTL_SECONDS_AFTER_FINISHED:-600}"
cleanup_job="${CLEANUP_JOB:-1}"
job_name_input="${JOB_NAME:-bootstrap-live-e2e-browser-users-$(date +%Y%m%d%H%M%S)}"
job_name="$(printf '%s' "$job_name_input" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9-' '-' | sed 's/^-//; s/-$//' | cut -c1-63)"

if [[ -z "$job_name" ]]; then
  echo "JOB_NAME resolved to empty value" >&2
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
  "-tenant-id" "$tenant_id"
  "-output" "$output"
)

if [[ -n "$tenant_name" ]]; then
  args+=( "-tenant-name" "$tenant_name" )
fi
if [[ -n "${PLAYWRIGHT_LIVE_BASE_URL:-}" ]]; then
  args+=( "-base-url" "$PLAYWRIGHT_LIVE_BASE_URL" )
fi
if [[ -n "${PLAYWRIGHT_LIVE_API_BASE_URL:-}" ]]; then
  args+=( "-api-base-url" "$PLAYWRIGHT_LIVE_API_BASE_URL" )
fi
if [[ -n "${PLATFORM_EMAIL:-}" ]]; then
  args+=( "-platform-email" "$PLATFORM_EMAIL" )
fi
if [[ -n "${PLATFORM_DISPLAY_NAME:-}" ]]; then
  args+=( "-platform-display-name" "$PLATFORM_DISPLAY_NAME" )
fi
if [[ -n "${PLATFORM_PASSWORD:-}" ]]; then
  args+=( "-platform-password" "$PLATFORM_PASSWORD" )
fi
if [[ -n "${WRITER_EMAIL:-}" ]]; then
  args+=( "-writer-email" "$WRITER_EMAIL" )
fi
if [[ -n "${WRITER_DISPLAY_NAME:-}" ]]; then
  args+=( "-writer-display-name" "$WRITER_DISPLAY_NAME" )
fi
if [[ -n "${WRITER_PASSWORD:-}" ]]; then
  args+=( "-writer-password" "$WRITER_PASSWORD" )
fi
if [[ -n "${READER_EMAIL:-}" ]]; then
  args+=( "-reader-email" "$READER_EMAIL" )
fi
if [[ -n "${READER_DISPLAY_NAME:-}" ]]; then
  args+=( "-reader-display-name" "$READER_DISPLAY_NAME" )
fi
if [[ -n "${READER_PASSWORD:-}" ]]; then
  args+=( "-reader-password" "$READER_PASSWORD" )
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
        app.kubernetes.io/component: live-e2e-browser-users
    spec:
      restartPolicy: Never
      serviceAccountName: ${service_account}
      containers:
        - name: bootstrap-live-e2e-browser-users
          image: ${image}
          imagePullPolicy: IfNotPresent
          command: ["/app/bootstrap_live_e2e_browser_users"]
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
