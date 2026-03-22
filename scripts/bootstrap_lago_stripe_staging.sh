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
LAGO_BOOTSTRAP_JOB_TIMEOUT="${LAGO_BOOTSTRAP_JOB_TIMEOUT:-180s}"
LAGO_BOOTSTRAP_JOB_TTL_SECONDS="${LAGO_BOOTSTRAP_JOB_TTL_SECONDS:-600}"
LAGO_BOOTSTRAP_CLEANUP_JOB="${LAGO_BOOTSTRAP_CLEANUP_JOB:-1}"
LAGO_BOOTSTRAP_JOB_NAME="${LAGO_BOOTSTRAP_JOB_NAME:-}"
LAGO_ORG_ID="${LAGO_ORG_ID:-}"
LAGO_ORG_NAME="${LAGO_ORG_NAME:-Usage Billing Control Plane Staging}"

STRIPE_PROVIDER_CODE="${STRIPE_PROVIDER_CODE:-}"
STRIPE_PROVIDER_NAME="${STRIPE_PROVIDER_NAME:-}"
STRIPE_SECRET_KEY="${STRIPE_SECRET_KEY:-}"
STRIPE_SUCCESS_REDIRECT_URL="${STRIPE_SUCCESS_REDIRECT_URL:-https://staging.sagarwaidande.org}"

LAGO_WEBHOOK_URL="${LAGO_WEBHOOK_URL:-https://api-staging.sagarwaidande.org/internal/lago/webhooks}"
LAGO_WEBHOOK_SIGNATURE_ALGO="${LAGO_WEBHOOK_SIGNATURE_ALGO:-hmac}"

STAGING_CONTACT_EMAIL="${STAGING_CONTACT_EMAIL:-sagar10018233@gmail.com}"
STAGING_CONTACT_FIRSTNAME="${STAGING_CONTACT_FIRSTNAME:-sagar}"
STAGING_CONTACT_LASTNAME="${STAGING_CONTACT_LASTNAME:-waidande}"
STAGING_CURRENCY="${STAGING_CURRENCY:-USD}"

RUN_ID="${RUN_ID:-$(date -u +%Y%m%d%H%M%S)-$RANDOM}"
SUCCESS_CUSTOMER_EXTERNAL_ID="${SUCCESS_CUSTOMER_EXTERNAL_ID:-cust_payment_smoke_success_${RUN_ID}}"
FAILURE_CUSTOMER_EXTERNAL_ID="${FAILURE_CUSTOMER_EXTERNAL_ID:-cust_payment_smoke_failure_${RUN_ID}}"
SUCCESS_PAYMENT_METHOD_FIXTURE="${SUCCESS_PAYMENT_METHOD_FIXTURE:-pm_card_visa}"

SUCCESS_ADDRESS_LINE1="${SUCCESS_ADDRESS_LINE1:-123 Test Street}"
SUCCESS_CITY="${SUCCESS_CITY:-New York}"
SUCCESS_STATE="${SUCCESS_STATE:-NY}"
SUCCESS_ZIPCODE="${SUCCESS_ZIPCODE:-10001}"
SUCCESS_COUNTRY="${SUCCESS_COUNTRY:-US}"

ruby_file="$(mktemp)"
manifest_file="$(mktemp)"
cleanup() {
  rm -f "$ruby_file"
  rm -f "$manifest_file"
}
trap cleanup EXIT

deployment_json="$(kubectl -n "$LAGO_NAMESPACE" get deploy "$LAGO_API_DEPLOYMENT" -o json)"
image="$(printf '%s' "$deployment_json" | jq -r '.spec.template.spec.containers[0].image // ""')"

if [[ -z "$image" ]]; then
  echo "failed to derive runtime image from deployment $LAGO_API_DEPLOYMENT in namespace $LAGO_NAMESPACE" >&2
  exit 1
fi

job_name_input="${LAGO_BOOTSTRAP_JOB_NAME:-lago-payment-bootstrap-$(date +%Y%m%d%H%M%S)}"
job_name="$(printf '%s' "$job_name_input" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9-' '-' | sed 's/^-//; s/-$//' | cut -c1-63)"

if [[ -z "$job_name" ]]; then
  echo "resolved empty Lago bootstrap job name" >&2
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

def ensure_result!(result, label)
  return result if result.respond_to?(:success?) && result.success?

  detail =
    if result.respond_to?(:error) && result.error
      result.error.inspect
    else
      result.inspect
    end
  raise "#{label} failed: #{detail}"
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
end

def upsert_customer!(organization:, billing_entity_code:, provider:, external_id:, name:, email:, firstname:, lastname:, currency:, address:)
  customer = Customer.find_by(external_id: external_id, organization_id: organization.id)

  service_args = {
    name: name,
    email: email,
    firstname: firstname,
    lastname: lastname,
    currency: currency,
    country: address[:country],
    address_line1: address[:address_line1],
    city: address[:city],
    state: address[:state],
    zipcode: address[:zipcode],
    payment_provider: "stripe",
    payment_provider_code: provider.code,
    provider_customer: {
      payment_provider: "stripe",
      payment_provider_code: provider.code,
      provider_payment_methods: ["card"],
      sync: true,
      sync_with_provider: true
    }
  }

  if customer
    result = Customers::UpdateService.call(customer: customer, args: service_args)
    ensure_result!(result, "update customer #{external_id}")
  else
    result = Customers::CreateService.call(
      organization_id: organization.id,
      billing_entity_code: billing_entity_code,
      external_id: external_id,
      **service_args
    )
    ensure_result!(result, "create customer #{external_id}")
    customer = result.customer
  end

  customer.reload
end

org =
  if env("LAGO_ORG_ID")
    Organization.find(env!("LAGO_ORG_ID"))
  else
    Organization.find_by!(name: env!("LAGO_ORG_NAME"))
  end

billing_entity_code = org.billing_entities.first&.code
raise "organization #{org.id} has no billing entity" if billing_entity_code.nil? || billing_entity_code.empty?

requested_provider_code = env("STRIPE_PROVIDER_CODE")
provider =
  if requested_provider_code.present?
    PaymentProviders::StripeProvider.find_by(organization_id: org.id, code: requested_provider_code)
  else
    existing_providers = PaymentProviders::StripeProvider.where(organization_id: org.id).order(created_at: :desc).to_a
    existing_providers.find { |candidate| candidate.code.to_s.start_with?("alpha_stripe_") } ||
      existing_providers.find { |candidate| candidate.code.to_s == "stripe_test" } ||
      existing_providers.first
  end

if provider.nil?
  provider_code = requested_provider_code.presence || "stripe_test"
  provider_name = env("STRIPE_PROVIDER_NAME").presence || "Stripe Test"
  secret_key = env!("STRIPE_SECRET_KEY")
  result = PaymentProviders::StripeService.new.create_or_update(
    organization_id: org.id,
    code: provider_code,
    name: provider_name,
    secret_key: secret_key,
    success_redirect_url: env("STRIPE_SUCCESS_REDIRECT_URL"),
    supports_3ds: false
  )
  ensure_result!(result, "create stripe provider")
  provider = result.stripe_provider
else
  provider.name = env("STRIPE_PROVIDER_NAME").presence || provider.name
  provider.success_redirect_url = env("STRIPE_SUCCESS_REDIRECT_URL").presence || provider.success_redirect_url
  if !env("STRIPE_SECRET_KEY").to_s.empty? && !provider.secret_key.present?
    provider.secret_key = env("STRIPE_SECRET_KEY")
  end
  provider.save!
end

raise "stripe provider #{provider.code} is missing secret_key" unless provider.secret_key.present?

webhook_endpoint = org.webhook_endpoints.find_by(webhook_url: env!("LAGO_WEBHOOK_URL"))
if webhook_endpoint.nil?
  result = WebhookEndpoints::CreateService.call(
    organization: org,
    params: {
      webhook_url: env!("LAGO_WEBHOOK_URL"),
      signature_algo: env("LAGO_WEBHOOK_SIGNATURE_ALGO", "hmac")
    }
  )
  ensure_result!(result, "create webhook endpoint")
  webhook_endpoint = result.webhook_endpoint
elsif webhook_endpoint.signature_algo.to_s != env("LAGO_WEBHOOK_SIGNATURE_ALGO", "hmac")
  webhook_endpoint.update!(signature_algo: env("LAGO_WEBHOOK_SIGNATURE_ALGO", "hmac"))
end

success_customer = upsert_customer!(
  organization: org,
  billing_entity_code: billing_entity_code,
  provider: provider,
  external_id: env!("SUCCESS_CUSTOMER_EXTERNAL_ID"),
  name: env!("SUCCESS_CUSTOMER_EXTERNAL_ID"),
  email: env!("STAGING_CONTACT_EMAIL"),
  firstname: env!("STAGING_CONTACT_FIRSTNAME"),
  lastname: env!("STAGING_CONTACT_LASTNAME"),
  currency: env("STAGING_CURRENCY", "USD"),
  address: {
    address_line1: env!("SUCCESS_ADDRESS_LINE1"),
    city: env!("SUCCESS_CITY"),
    state: env!("SUCCESS_STATE"),
    zipcode: env!("SUCCESS_ZIPCODE"),
    country: env!("SUCCESS_COUNTRY")
  }
)
sync_customer_to_stripe!(provider: provider, customer: success_customer)
success_payment_method_id = ensure_success_payment_method!(
  provider: provider,
  customer: success_customer,
  fixture_id: env("SUCCESS_PAYMENT_METHOD_FIXTURE", "pm_card_visa")
)

failure_customer = upsert_customer!(
  organization: org,
  billing_entity_code: billing_entity_code,
  provider: provider,
  external_id: env!("FAILURE_CUSTOMER_EXTERNAL_ID"),
  name: env!("FAILURE_CUSTOMER_EXTERNAL_ID"),
  email: env!("STAGING_CONTACT_EMAIL"),
  firstname: env!("STAGING_CONTACT_FIRSTNAME"),
  lastname: env!("STAGING_CONTACT_LASTNAME"),
  currency: env("STAGING_CURRENCY", "USD"),
  address: {
    address_line1: nil,
    city: nil,
    state: nil,
    zipcode: nil,
    country: nil
  }
)
ensure_failure_without_payment_method!(provider: provider, customer: failure_customer)

summary = {
  run_id: env!("RUN_ID"),
  organization: {
    id: org.id,
    name: org.name,
    hmac_key: org.hmac_key
  },
  stripe_provider: {
    id: provider.id,
    code: provider.code,
    success_redirect_url: provider.success_redirect_url
  },
  webhook_endpoint: {
    id: webhook_endpoint.id,
    webhook_url: webhook_endpoint.webhook_url,
    signature_algo: webhook_endpoint.signature_algo
  },
  customers: {
    success: {
      external_id: success_customer.external_id,
      stripe_customer_id: success_customer.stripe_customer&.provider_customer_id,
      payment_method_id: success_payment_method_id,
      address_synced: {
        address_line1: success_customer.address_line1,
        city: success_customer.city,
        state: success_customer.state,
        zipcode: success_customer.zipcode,
        country: success_customer.country
      }
    },
    failure: {
      external_id: failure_customer.external_id,
      stripe_customer_id: failure_customer.stripe_customer&.provider_customer_id,
      payment_method_id: failure_customer.stripe_customer&.payment_method_id
    }
  },
  notes: [
    "failure customer is intentionally left without a default payment method to force a deterministic failed retry path",
    "success customer is synced with a billing address because India-based Stripe accounts require customer address data for export transactions"
  ]
}

puts JSON.pretty_generate(summary)
RUBY

custom_env_json="$(jq -n \
  --arg run_id "$RUN_ID" \
  --arg lago_org_id "$LAGO_ORG_ID" \
  --arg lago_org_name "$LAGO_ORG_NAME" \
  --arg stripe_provider_code "$STRIPE_PROVIDER_CODE" \
  --arg stripe_provider_name "$STRIPE_PROVIDER_NAME" \
  --arg stripe_secret_key "$STRIPE_SECRET_KEY" \
  --arg stripe_success_redirect_url "$STRIPE_SUCCESS_REDIRECT_URL" \
  --arg lago_webhook_url "$LAGO_WEBHOOK_URL" \
  --arg lago_webhook_signature_algo "$LAGO_WEBHOOK_SIGNATURE_ALGO" \
  --arg staging_contact_email "$STAGING_CONTACT_EMAIL" \
  --arg staging_contact_firstname "$STAGING_CONTACT_FIRSTNAME" \
  --arg staging_contact_lastname "$STAGING_CONTACT_LASTNAME" \
  --arg staging_currency "$STAGING_CURRENCY" \
  --arg success_customer_external_id "$SUCCESS_CUSTOMER_EXTERNAL_ID" \
  --arg failure_customer_external_id "$FAILURE_CUSTOMER_EXTERNAL_ID" \
  --arg success_payment_method_fixture "$SUCCESS_PAYMENT_METHOD_FIXTURE" \
  --arg success_address_line1 "$SUCCESS_ADDRESS_LINE1" \
  --arg success_city "$SUCCESS_CITY" \
  --arg success_state "$SUCCESS_STATE" \
  --arg success_zipcode "$SUCCESS_ZIPCODE" \
  --arg success_country "$SUCCESS_COUNTRY" \
  '[
    {name: "RUN_ID", value: $run_id},
    {name: "LAGO_ORG_ID", value: $lago_org_id},
    {name: "LAGO_ORG_NAME", value: $lago_org_name},
    {name: "STRIPE_PROVIDER_CODE", value: $stripe_provider_code},
    {name: "STRIPE_PROVIDER_NAME", value: $stripe_provider_name},
    {name: "STRIPE_SECRET_KEY", value: $stripe_secret_key},
    {name: "STRIPE_SUCCESS_REDIRECT_URL", value: $stripe_success_redirect_url},
    {name: "LAGO_WEBHOOK_URL", value: $lago_webhook_url},
    {name: "LAGO_WEBHOOK_SIGNATURE_ALGO", value: $lago_webhook_signature_algo},
    {name: "STAGING_CONTACT_EMAIL", value: $staging_contact_email},
    {name: "STAGING_CONTACT_FIRSTNAME", value: $staging_contact_firstname},
    {name: "STAGING_CONTACT_LASTNAME", value: $staging_contact_lastname},
    {name: "STAGING_CURRENCY", value: $staging_currency},
    {name: "SUCCESS_CUSTOMER_EXTERNAL_ID", value: $success_customer_external_id},
    {name: "FAILURE_CUSTOMER_EXTERNAL_ID", value: $failure_customer_external_id},
    {name: "SUCCESS_PAYMENT_METHOD_FIXTURE", value: $success_payment_method_fixture},
    {name: "SUCCESS_ADDRESS_LINE1", value: $success_address_line1},
    {name: "SUCCESS_CITY", value: $success_city},
    {name: "SUCCESS_STATE", value: $success_state},
    {name: "SUCCESS_ZIPCODE", value: $success_zipcode},
    {name: "SUCCESS_COUNTRY", value: $success_country}
  ]')"

printf '%s' "$deployment_json" | jq \
  --arg job_name "$job_name" \
  --arg namespace "$LAGO_NAMESPACE" \
  --argjson ttl "$LAGO_BOOTSTRAP_JOB_TTL_SECONDS" \
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
                "app.kubernetes.io/component": "payment-bootstrap"
              }
            },
            spec: {
              restartPolicy: "Never",
              serviceAccountName: ($deploy.spec.template.spec.serviceAccountName // null),
              imagePullSecrets: ($deploy.spec.template.spec.imagePullSecrets // null),
              containers: [
                {
                  name: "lago-payment-bootstrap",
                  image: $deploy.spec.template.spec.containers[0].image,
                  imagePullPolicy: ($deploy.spec.template.spec.containers[0].imagePullPolicy // "IfNotPresent"),
                  command: ["sh", "-lc"],
                  args: [
                    "cat >/tmp/bootstrap_lago_stripe_staging.rb <<'"'"'RUBY'"'"'\n" +
                    $ruby_script +
                    "\nRUBY\ncd /app\nbin/rails runner /tmp/bootstrap_lago_stripe_staging.rb"
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

if ! kubectl -n "$LAGO_NAMESPACE" wait --for=condition=complete "job/${job_name}" --timeout="$LAGO_BOOTSTRAP_JOB_TIMEOUT" >/dev/null; then
  echo "Lago payment bootstrap job failed. This indicates staging Lago runtime or DB wiring is broken, not an Alpha payment-surface regression." >&2
  kubectl -n "$LAGO_NAMESPACE" describe "job/${job_name}" >&2 || true
  kubectl -n "$LAGO_NAMESPACE" logs "job/${job_name}" >&2 || true
  exit 1
fi

kubectl -n "$LAGO_NAMESPACE" logs "job/${job_name}"

if [[ "$LAGO_BOOTSTRAP_CLEANUP_JOB" == "1" ]]; then
  kubectl -n "$LAGO_NAMESPACE" delete "job/${job_name}" --ignore-not-found >/dev/null
fi
