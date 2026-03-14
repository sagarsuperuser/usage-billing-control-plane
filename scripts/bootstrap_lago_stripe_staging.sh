#!/usr/bin/env bash
set -euo pipefail

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

require_cmd kubectl
require_cmd mktemp

LAGO_NAMESPACE="${LAGO_NAMESPACE:-lago}"
LAGO_API_DEPLOYMENT="${LAGO_API_DEPLOYMENT:-lago-api}"
LAGO_ORG_ID="${LAGO_ORG_ID:-}"
LAGO_ORG_NAME="${LAGO_ORG_NAME:-Usage Billing Control Plane Staging}"

STRIPE_PROVIDER_CODE="${STRIPE_PROVIDER_CODE:-stripe_test}"
STRIPE_PROVIDER_NAME="${STRIPE_PROVIDER_NAME:-Stripe Test}"
STRIPE_SECRET_KEY="${STRIPE_SECRET_KEY:-}"
STRIPE_SUCCESS_REDIRECT_URL="${STRIPE_SUCCESS_REDIRECT_URL:-https://staging.sagarwaidande.org}"

LAGO_WEBHOOK_URL="${LAGO_WEBHOOK_URL:-https://api-staging.sagarwaidande.org/internal/lago/webhooks}"
LAGO_WEBHOOK_SIGNATURE_ALGO="${LAGO_WEBHOOK_SIGNATURE_ALGO:-jwt}"

STAGING_CONTACT_EMAIL="${STAGING_CONTACT_EMAIL:-sagar10018233@gmail.com}"
STAGING_CONTACT_FIRSTNAME="${STAGING_CONTACT_FIRSTNAME:-sagar}"
STAGING_CONTACT_LASTNAME="${STAGING_CONTACT_LASTNAME:-waidande}"
STAGING_CURRENCY="${STAGING_CURRENCY:-USD}"

SUCCESS_CUSTOMER_EXTERNAL_ID="${SUCCESS_CUSTOMER_EXTERNAL_ID:-cust_e2e_success}"
FAILURE_CUSTOMER_EXTERNAL_ID="${FAILURE_CUSTOMER_EXTERNAL_ID:-cust_e2e_failure}"
SUCCESS_PAYMENT_METHOD_FIXTURE="${SUCCESS_PAYMENT_METHOD_FIXTURE:-pm_card_visa}"

SUCCESS_ADDRESS_LINE1="${SUCCESS_ADDRESS_LINE1:-123 Test Street}"
SUCCESS_CITY="${SUCCESS_CITY:-New York}"
SUCCESS_STATE="${SUCCESS_STATE:-NY}"
SUCCESS_ZIPCODE="${SUCCESS_ZIPCODE:-10001}"
SUCCESS_COUNTRY="${SUCCESS_COUNTRY:-US}"

ruby_file="$(mktemp)"
cleanup() {
  rm -f "$ruby_file"
}
trap cleanup EXIT

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

provider = PaymentProviders::StripeProvider.find_by(organization_id: org.id, code: env!("STRIPE_PROVIDER_CODE"))

if provider.nil?
  secret_key = env!("STRIPE_SECRET_KEY")
  result = PaymentProviders::StripeService.new.create_or_update(
    organization_id: org.id,
    code: env!("STRIPE_PROVIDER_CODE"),
    name: env!("STRIPE_PROVIDER_NAME"),
    secret_key: secret_key,
    success_redirect_url: env("STRIPE_SUCCESS_REDIRECT_URL"),
    supports_3ds: false
  )
  ensure_result!(result, "create stripe provider")
  provider = result.stripe_provider
else
  provider.name = env!("STRIPE_PROVIDER_NAME")
  provider.success_redirect_url = env("STRIPE_SUCCESS_REDIRECT_URL")
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
      signature_algo: env("LAGO_WEBHOOK_SIGNATURE_ALGO", "jwt")
    }
  )
  ensure_result!(result, "create webhook endpoint")
  webhook_endpoint = result.webhook_endpoint
elsif webhook_endpoint.signature_algo.to_s != env("LAGO_WEBHOOK_SIGNATURE_ALGO", "jwt")
  webhook_endpoint.update!(signature_algo: env("LAGO_WEBHOOK_SIGNATURE_ALGO", "jwt"))
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
  organization: {
    id: org.id,
    name: org.name
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

kubectl -n "$LAGO_NAMESPACE" exec -i "deploy/$LAGO_API_DEPLOYMENT" -- \
  env \
  "LAGO_ORG_ID=$LAGO_ORG_ID" \
  "LAGO_ORG_NAME=$LAGO_ORG_NAME" \
  "STRIPE_PROVIDER_CODE=$STRIPE_PROVIDER_CODE" \
  "STRIPE_PROVIDER_NAME=$STRIPE_PROVIDER_NAME" \
  "STRIPE_SECRET_KEY=$STRIPE_SECRET_KEY" \
  "STRIPE_SUCCESS_REDIRECT_URL=$STRIPE_SUCCESS_REDIRECT_URL" \
  "LAGO_WEBHOOK_URL=$LAGO_WEBHOOK_URL" \
  "LAGO_WEBHOOK_SIGNATURE_ALGO=$LAGO_WEBHOOK_SIGNATURE_ALGO" \
  "STAGING_CONTACT_EMAIL=$STAGING_CONTACT_EMAIL" \
  "STAGING_CONTACT_FIRSTNAME=$STAGING_CONTACT_FIRSTNAME" \
  "STAGING_CONTACT_LASTNAME=$STAGING_CONTACT_LASTNAME" \
  "STAGING_CURRENCY=$STAGING_CURRENCY" \
  "SUCCESS_CUSTOMER_EXTERNAL_ID=$SUCCESS_CUSTOMER_EXTERNAL_ID" \
  "FAILURE_CUSTOMER_EXTERNAL_ID=$FAILURE_CUSTOMER_EXTERNAL_ID" \
  "SUCCESS_PAYMENT_METHOD_FIXTURE=$SUCCESS_PAYMENT_METHOD_FIXTURE" \
  "SUCCESS_ADDRESS_LINE1=$SUCCESS_ADDRESS_LINE1" \
  "SUCCESS_CITY=$SUCCESS_CITY" \
  "SUCCESS_STATE=$SUCCESS_STATE" \
  "SUCCESS_ZIPCODE=$SUCCESS_ZIPCODE" \
  "SUCCESS_COUNTRY=$SUCCESS_COUNTRY" \
  sh -lc 'cat >/tmp/bootstrap_lago_stripe_staging.rb && cd /app && bin/rails runner /tmp/bootstrap_lago_stripe_staging.rb' \
  <"$ruby_file"
