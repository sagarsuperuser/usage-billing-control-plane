#!/usr/bin/env bash
set -euo pipefail

namespace="${LAGO_NAMESPACE:-lago}"
service_account_name="${LAGO_SERVICE_ACCOUNT_NAME:-lago-serviceaccount}"
role_arn="${LAGO_SERVICE_ACCOUNT_ROLE_ARN:-}"

if [[ -z "$service_account_name" ]]; then
  echo "LAGO_SERVICE_ACCOUNT_NAME must not be empty" >&2
  exit 1
fi

annotations=""
if [[ -n "$role_arn" ]]; then
  annotations=$(cat <<EOF
  annotations:
    eks.amazonaws.com/role-arn: "$role_arn"
EOF
)
fi

kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null

cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${service_account_name}
  namespace: ${namespace}
${annotations}
EOF

printf '[pass] Lago service account ensured: %s/%s\n' "$namespace" "$service_account_name"
if [[ -n "$role_arn" ]]; then
  printf '  role: %s\n' "$role_arn"
fi
