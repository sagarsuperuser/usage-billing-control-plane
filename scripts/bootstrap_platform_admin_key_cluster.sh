#!/usr/bin/env bash
set -euo pipefail

namespace="${NAMESPACE:-lago-alpha}"
release_name="${RELEASE_NAME:-lago-alpha}"
api_deployment="${API_DEPLOYMENT_NAME:-${release_name}-lago-alpha-api}"
key_name="${PLATFORM_KEY_NAME:-${KEY_NAME:-bootstrap-platform-admin}}"
output="${OUTPUT:-json}"
allow_existing_active_keys="${ALLOW_EXISTING_ACTIVE_KEYS:-1}"
job_timeout="${JOB_TIMEOUT:-180s}"
ttl_seconds="${TTL_SECONDS_AFTER_FINISHED:-600}"
cleanup_job="${CLEANUP_JOB:-1}"

job_name_input="${JOB_NAME:-platform-admin-bootstrap-$(date +%Y%m%d%H%M%S)}"
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

args=( "-name" "$key_name" "-output" "$output" )
if [[ "$allow_existing_active_keys" == "1" ]]; then
  args+=( "-allow-existing-active-keys" )
fi
if [[ -n "${EXPIRES_AT:-}" ]]; then
  args+=( "-expires-at" "$EXPIRES_AT" )
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
        app.kubernetes.io/component: admin-bootstrap
    spec:
      restartPolicy: Never
      serviceAccountName: ${service_account}
      containers:
        - name: bootstrap-platform-admin
          image: ${image}
          imagePullPolicy: IfNotPresent
          command: ["/app/bootstrap_platform_admin_key"]
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
