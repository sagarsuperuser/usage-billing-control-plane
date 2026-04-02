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
job_name_input="${JOB_NAME:-mint-live-e2e-keys-$(date +%Y%m%d%H%M%S)}"
job_name="$(printf '%s' "$job_name_input" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9-' '-' | sed 's/^-//; s/-$//' | cut -c1-63)"

if [[ -z "$job_name" ]]; then
  echo "JOB_NAME resolved to empty value" >&2
  exit 1
fi

image="$(kubectl -n "$namespace" get deploy "$api_deployment" -o jsonpath='{.spec.template.spec.containers[0].image}')"
service_account="$(kubectl -n "$namespace" get deploy "$api_deployment" -o jsonpath='{.spec.template.spec.serviceAccountName}')"
config_map_ref="$(kubectl -n "$namespace" get deploy "$api_deployment" -o jsonpath='{.spec.template.spec.containers[0].envFrom[?(@.configMapRef)].configMapRef.name}')"
secret_refs=()
while IFS= read -r line; do
  [[ -n "$line" ]] && secret_refs+=("$line")
done < <(kubectl -n "$namespace" get deploy "$api_deployment" -o jsonpath='{range .spec.template.spec.containers[0].envFrom[?(@.secretRef)]}{.secretRef.name}{"\n"}{end}')

if [[ -z "$image" || -z "$service_account" || -z "$config_map_ref" || "${#secret_refs[@]}" -eq 0 ]]; then
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
if [[ -n "${EXPIRES_AT:-}" ]]; then
  args+=( "-expires-at" "$EXPIRES_AT" )
fi
if [[ -n "${PLATFORM_KEY_NAME:-}" ]]; then
  args+=( "-platform-key-name" "$PLATFORM_KEY_NAME" )
fi
if [[ -n "${WRITER_KEY_NAME:-}" ]]; then
  args+=( "-writer-key-name" "$WRITER_KEY_NAME" )
fi
if [[ -n "${READER_KEY_NAME:-}" ]]; then
  args+=( "-reader-key-name" "$READER_KEY_NAME" )
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
        app.kubernetes.io/component: live-e2e-key-mint
    spec:
      restartPolicy: Never
      serviceAccountName: ${service_account}
      containers:
        - name: mint-live-e2e-keys
          image: ${image}
          imagePullPolicy: IfNotPresent
          command: ["/app/mint_live_e2e_keys"]
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
EOF
for sr in "${secret_refs[@]}"; do
  cat >>"$manifest_file" <<EOF
            - secretRef:
                name: ${sr}
EOF
done

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
