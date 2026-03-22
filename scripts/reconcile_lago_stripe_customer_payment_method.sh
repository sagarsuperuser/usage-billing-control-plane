#!/usr/bin/env bash
set -euo pipefail

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

require_cmd kubectl
require_cmd jq
require_cmd mktemp

LAGO_NAMESPACE="${LAGO_NAMESPACE:-lago}"
LAGO_API_DEPLOYMENT="${LAGO_API_DEPLOYMENT:-lago-api}"
LAGO_PAYMENT_METHOD_JOB_TIMEOUT="${LAGO_PAYMENT_METHOD_JOB_TIMEOUT:-180s}"
LAGO_PAYMENT_METHOD_JOB_TTL_SECONDS="${LAGO_PAYMENT_METHOD_JOB_TTL_SECONDS:-600}"
LAGO_PAYMENT_METHOD_CLEANUP_JOB="${LAGO_PAYMENT_METHOD_CLEANUP_JOB:-1}"
LAGO_PAYMENT_METHOD_JOB_NAME="${LAGO_PAYMENT_METHOD_JOB_NAME:-}"
LAGO_ORG_ID="${LAGO_ORG_ID:-}"
LAGO_ORG_NAME="${LAGO_ORG_NAME:-Usage Billing Control Plane Staging}"
STRIPE_PROVIDER_CODE="${STRIPE_PROVIDER_CODE:-}"
CUSTOMER_EXTERNAL_ID="${CUSTOMER_EXTERNAL_ID:-}"
PAYMENT_METHOD_ACTION="${PAYMENT_METHOD_ACTION:-attach_default}"
PAYMENT_METHOD_FIXTURE="${PAYMENT_METHOD_FIXTURE:-pm_card_visa}"

if [[ -z "$CUSTOMER_EXTERNAL_ID" ]]; then
  echo "CUSTOMER_EXTERNAL_ID is required" >&2
  exit 1
fi

case "$PAYMENT_METHOD_ACTION" in
  attach_default|detach_all)
    ;;
  *)
    echo "PAYMENT_METHOD_ACTION must be one of: attach_default, detach_all" >&2
    exit 1
    ;;
esac

ruby_file="$(mktemp)"
manifest_file="$(mktemp)"
cleanup() {
  rm -f "$ruby_file" "$manifest_file"
}
trap cleanup EXIT

deployment_json="$(kubectl -n "$LAGO_NAMESPACE" get deploy "$LAGO_API_DEPLOYMENT" -o json)"
image="$(printf '%s' "$deployment_json" | jq -r '.spec.template.spec.containers[0].image // ""')"

if [[ -z "$image" ]]; then
  echo "failed to derive runtime image from deployment $LAGO_API_DEPLOYMENT in namespace $LAGO_NAMESPACE" >&2
  exit 1
fi

job_name_input="${LAGO_PAYMENT_METHOD_JOB_NAME:-lago-payment-method-reconcile-$(date +%Y%m%d%H%M%S)}"
job_name="$(printf '%s' "$job_name_input" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9-' '-' | sed 's/^-//; s/-$//' | cut -c1-63)"

if [[ -z "$job_name" ]]; then
  echo "resolved empty Lago payment method reconcile job name" >&2
  exit 1
fi

cat >"$ruby_file" <<'RUBY'
require "json"

def env(key, default = nil)
  value = ENV[key]
  return default if value.nil? || value.empty?

  value
end

def env!(key)
  value = env(key)
  raise "missing required environment variable: #{key}" if value.nil? || value.empty?

  value
end

def sync_customer_to_stripe!(provider:, customer:)
  stripe_customer = customer.stripe_customer
  raise "customer #{customer.external_id} is missing stripe_customer" unless stripe_customer&.provider_customer_id.present?

  Stripe::Customer.update(
    stripe_customer.provider_customer_id,
    {
      name: customer.name,
      email: customer.email,
      address: {
        line1: customer.address_line1,
        line2: customer.address_line2,
        city: customer.city,
        state: customer.state,
        postal_code: customer.zipcode,
        country: customer.country
      }
    },
    {api_key: provider.secret_key}
  )
end

def ensure_success_payment_method!(provider:, customer:, fixture_id:)
  stripe_customer = customer.stripe_customer
  raise "customer #{customer.external_id} is missing stripe_customer" unless stripe_customer&.provider_customer_id.present?

  remote_customer = Stripe::Customer.retrieve(stripe_customer.provider_customer_id, {api_key: provider.secret_key})
  payment_methods = Stripe::Customer.list_payment_methods(
    stripe_customer.provider_customer_id,
    {limit: 10},
    {api_key: provider.secret_key}
  ).data

  payment_method_id = remote_customer["invoice_settings"]["default_payment_method"]
  payment_method_id ||= payment_methods.first&.id

  if payment_method_id.nil?
    attached = Stripe::PaymentMethod.attach(
      fixture_id,
      {customer: stripe_customer.provider_customer_id},
      {api_key: provider.secret_key}
    )
    payment_method_id = attached.id
  end

  Stripe::Customer.update(
    stripe_customer.provider_customer_id,
    {invoice_settings: {default_payment_method: payment_method_id}},
    {api_key: provider.secret_key}
  )

  PaymentProviderCustomers::Stripe::UpdatePaymentMethodService.call!(
    stripe_customer: stripe_customer,
    payment_method_id: payment_method_id
  )

  payment_method_id
end

def ensure_failure_without_payment_method!(provider:, customer:)
  stripe_customer = customer.stripe_customer
  raise "customer #{customer.external_id} is missing stripe_customer" unless stripe_customer&.provider_customer_id.present?

  payment_methods = Stripe::Customer.list_payment_methods(
    stripe_customer.provider_customer_id,
    {limit: 100},
    {api_key: provider.secret_key}
  ).data

  payment_methods.each do |payment_method|
    Stripe::PaymentMethod.detach(payment_method.id, {}, {api_key: provider.secret_key})
  rescue Stripe::InvalidRequestError
    nil
  end

  Stripe::Customer.update(
    stripe_customer.provider_customer_id,
    {invoice_settings: {default_payment_method: nil}},
    {api_key: provider.secret_key}
  )
  stripe_customer.update!(payment_method_id: nil)

  nil
end

org =
  if env("LAGO_ORG_ID")
    Organization.find(env!("LAGO_ORG_ID"))
  else
    Organization.find_by!(name: env!("LAGO_ORG_NAME"))
  end

requested_provider_code = env("STRIPE_PROVIDER_CODE")
provider =
  if requested_provider_code.present?
    PaymentProviders::StripeProvider.find_by!(organization_id: org.id, code: requested_provider_code)
  else
    existing_providers = PaymentProviders::StripeProvider.where(organization_id: org.id).order(created_at: :desc).to_a
    existing_providers.find { |candidate| candidate.code.to_s.start_with?("alpha_stripe_") } ||
      existing_providers.find { |candidate| candidate.code.to_s == "stripe_test" } ||
      existing_providers.first
  end

raise "missing stripe provider for organization #{org.id}" if provider.nil?
raise "stripe provider #{provider.code} is missing secret_key" unless provider.secret_key.present?

customer = Customer.find_by!(organization_id: org.id, external_id: env!("CUSTOMER_EXTERNAL_ID"))
sync_customer_to_stripe!(provider: provider, customer: customer)

payment_method_id =
  case env!("PAYMENT_METHOD_ACTION")
  when "attach_default"
    ensure_success_payment_method!(
      provider: provider,
      customer: customer,
      fixture_id: env("PAYMENT_METHOD_FIXTURE", "pm_card_visa")
    )
  when "detach_all"
    ensure_failure_without_payment_method!(provider: provider, customer: customer)
  else
    raise "unsupported PAYMENT_METHOD_ACTION=#{env("PAYMENT_METHOD_ACTION").inspect}"
  end

customer.reload
stripe_customer = customer.stripe_customer

puts JSON.pretty_generate(
  {
    organization: {
      id: org.id,
      name: org.name
    },
    stripe_provider: {
      code: provider.code
    },
    customer: {
      external_id: customer.external_id,
      stripe_customer_id: stripe_customer&.provider_customer_id,
      payment_method_id: payment_method_id,
      stored_payment_method_id: stripe_customer&.payment_method_id
    },
    action: env!("PAYMENT_METHOD_ACTION")
  }
)
RUBY

custom_env_json="$(jq -n \
  --arg lago_org_id "$LAGO_ORG_ID" \
  --arg lago_org_name "$LAGO_ORG_NAME" \
  --arg stripe_provider_code "$STRIPE_PROVIDER_CODE" \
  --arg customer_external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg payment_method_action "$PAYMENT_METHOD_ACTION" \
  --arg payment_method_fixture "$PAYMENT_METHOD_FIXTURE" \
  '[
    {name: "LAGO_ORG_ID", value: $lago_org_id},
    {name: "LAGO_ORG_NAME", value: $lago_org_name},
    {name: "STRIPE_PROVIDER_CODE", value: $stripe_provider_code},
    {name: "CUSTOMER_EXTERNAL_ID", value: $customer_external_id},
    {name: "PAYMENT_METHOD_ACTION", value: $payment_method_action},
    {name: "PAYMENT_METHOD_FIXTURE", value: $payment_method_fixture}
  ]')"

printf '%s' "$deployment_json" | jq \
  --arg job_name "$job_name" \
  --arg namespace "$LAGO_NAMESPACE" \
  --argjson ttl "$LAGO_PAYMENT_METHOD_JOB_TTL_SECONDS" \
  --argjson custom_env "$custom_env_json" \
  --rawfile ruby_script "$ruby_file" \
  '
    . as $deploy
    | {
        apiVersion: "batch/v1",
        kind: "Job",
        metadata: {
          name: $job_name,
          namespace: $namespace
        },
        spec: {
          ttlSecondsAfterFinished: $ttl,
          backoffLimit: 0,
          template: {
            metadata: {
              labels: {
                "app.kubernetes.io/name": "lago",
                "app.kubernetes.io/component": "payment-method-reconcile"
              }
            },
            spec: {
              restartPolicy: "Never",
              serviceAccountName: ($deploy.spec.template.spec.serviceAccountName // null),
              imagePullSecrets: ($deploy.spec.template.spec.imagePullSecrets // null),
              containers: [
                {
                  name: "lago-payment-method-reconcile",
                  image: $deploy.spec.template.spec.containers[0].image,
                  imagePullPolicy: ($deploy.spec.template.spec.containers[0].imagePullPolicy // "IfNotPresent"),
                  command: ["sh", "-lc"],
                  args: [
                    "cat >/tmp/reconcile_lago_stripe_customer_payment_method.rb <<'"'"'RUBY'"'"'\n" +
                    $ruby_script +
                    "\nRUBY\ncd /app\nbin/rails runner /tmp/reconcile_lago_stripe_customer_payment_method.rb"
                  ],
                  env: (($deploy.spec.template.spec.containers[0].env // []) + $custom_env)
                }
              ]
            }
          }
        }
      }
    | del(.. | nulls)
  ' >"$manifest_file"

kubectl apply -f "$manifest_file" >/dev/null

if ! kubectl -n "$LAGO_NAMESPACE" wait --for=condition=complete "job/${job_name}" --timeout="$LAGO_PAYMENT_METHOD_JOB_TIMEOUT" >/dev/null; then
  echo "Lago payment method reconcile job failed." >&2
  kubectl -n "$LAGO_NAMESPACE" describe "job/${job_name}" >&2 || true
  kubectl -n "$LAGO_NAMESPACE" logs "job/${job_name}" >&2 || true
  exit 1
fi

kubectl -n "$LAGO_NAMESPACE" logs "job/${job_name}"

if [[ "$LAGO_PAYMENT_METHOD_CLEANUP_JOB" == "1" ]]; then
  kubectl -n "$LAGO_NAMESPACE" delete "job/${job_name}" --ignore-not-found >/dev/null
fi
