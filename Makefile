SHELL := /bin/bash

GO ?= go
COMPOSE_FILE ?= docker-compose.postgres.yml
DATABASE_URL ?= postgres://postgres:postgres@localhost:15432/alpha?sslmode=disable
TEST_DATABASE_URL ?= postgres://postgres:postgres@localhost:15432/alpha_test?sslmode=disable
TEST_TEMPORAL_ADDRESS ?= 127.0.0.1:17233
TEST_TEMPORAL_NAMESPACE ?= default
CHECK_GITHUB ?= 0
RUN_GO_TESTS ?= 1
RUN_TERRAFORM_VALIDATE ?= 0
GITHUB_REPOSITORY ?=
TF_DIR ?= infra/terraform/aws
HELM_CHART ?= deploy/helm/alpha
ENVIRONMENT ?= staging
RELEASE_NAME ?= lago-alpha
NAMESPACE ?= lago-alpha
IMAGE_TAG ?= $(shell git rev-parse HEAD)
API_IMAGE_REPOSITORY ?=
WEB_IMAGE_REPOSITORY ?=
REVISION ?=

.DEFAULT_GOAL := help

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-20s %s\n", $$1, $$2}'

fmt: ## Format Go code
	@$(GO) fmt ./...

tidy: ## Tidy Go modules
	@$(GO) mod tidy

test: ## Run all tests
	@$(GO) test ./...

test-unit: ## Run fast unit tests
	@$(GO) test ./internal/domain

test-browser-mocked: web-e2e ## Run mocked browser/session workflow tests

test-smoke-local: test-real-env-smoke ## Run fast local controlled-env smoke

test-integration-local: test-integration ## Run full local integration workflow

verify-governance: ## Verify governance metadata (CODEOWNERS)
	@./scripts/verify_codeowners.sh

preflight-release: ## Run release preflight checks
	@ENVIRONMENT='$(ENVIRONMENT)' CHECK_GITHUB='$(CHECK_GITHUB)' RUN_GO_TESTS='$(RUN_GO_TESTS)' RUN_TERRAFORM_VALIDATE='$(RUN_TERRAFORM_VALIDATE)' GITHUB_REPOSITORY='$(GITHUB_REPOSITORY)' ./scripts/preflight_staging.sh

preflight-staging: ## Run release preflight checks for staging
	@ENVIRONMENT='staging' CHECK_GITHUB='$(CHECK_GITHUB)' RUN_GO_TESTS='$(RUN_GO_TESTS)' RUN_TERRAFORM_VALIDATE='$(RUN_TERRAFORM_VALIDATE)' GITHUB_REPOSITORY='$(GITHUB_REPOSITORY)' ./scripts/preflight_staging.sh

db-up: ## Start local Postgres via docker compose
	@docker compose -f $(COMPOSE_FILE) up -d

db-down: ## Stop local Postgres via docker compose
	@docker compose -f $(COMPOSE_FILE) down

db-ps: ## Show local Postgres container status
	@docker compose -f $(COMPOSE_FILE) ps

db-logs: ## Tail local Postgres logs
	@docker compose -f $(COMPOSE_FILE) logs -f postgres

wait-db: ## Wait for Postgres health status
	@./scripts/wait_for_postgres.sh $(COMPOSE_FILE) postgres 90

migrate: ## Run SQL migrations using cmd/migrate
	@DATABASE_URL='$(DATABASE_URL)' $(GO) run ./cmd/migrate

migrate-up: migrate ## Alias for migrate

migrate-status: ## Show migration status
	@DATABASE_URL='$(DATABASE_URL)' $(GO) run ./cmd/migrate status

migrate-verify: ## Verify no pending or unknown applied migrations remain
	@DATABASE_URL='$(DATABASE_URL)' $(GO) run ./cmd/migrate verify

run: ## Start API server
	@DATABASE_URL='$(DATABASE_URL)' $(GO) run ./cmd/server

bootstrap-platform-admin-key: ## Bootstrap the first platform admin API key
	@bash ./scripts/bootstrap_platform_admin_key.sh

bootstrap-platform-admin-key-cluster: ## Bootstrap a platform admin API key from inside the cluster
	@bash ./scripts/bootstrap_platform_admin_key_cluster.sh

mint-live-e2e-keys-cluster: ## Mint fresh API keys for live verification from inside the cluster
	@bash ./scripts/mint_live_e2e_keys_cluster.sh

bootstrap-live-e2e-browser-users-cluster: ## Ensure fresh live browser users for Playwright staging smoke
	@bash ./scripts/bootstrap_live_e2e_browser_users_cluster.sh

cleanup-staging-flow-data: ## Dry-run or apply cleanup of stale staging flow fixtures from inside the cluster
	@ENVIRONMENT='$(ENVIRONMENT)' OUTPUT='$(OUTPUT)' APPLY='$(APPLY)' CONFIRM_STAGING_FLOW_CLEANUP='$(CONFIRM_STAGING_FLOW_CLEANUP)' INCLUDE_REPLAY_FIXTURES='$(INCLUDE_REPLAY_FIXTURES)' INCLUDE_PAYMENT_FIXTURES='$(INCLUDE_PAYMENT_FIXTURES)' INCLUDE_LIVE_BROWSER_FIXTURES='$(INCLUDE_LIVE_BROWSER_FIXTURES)' bash ./scripts/cleanup_staging_flow_data_cluster.sh

cleanup-staging-flow-data-local: ## Legacy local psql cleanup path
	@DATABASE_URL='$(DATABASE_URL)' ENVIRONMENT='$(ENVIRONMENT)' APPLY='$(APPLY)' CONFIRM_STAGING_FLOW_CLEANUP='$(CONFIRM_STAGING_FLOW_CLEANUP)' bash ./scripts/cleanup_staging_flow_data.sh

temporal-staging-deploy: ## Deploy Temporal staging into the current cluster
	@./scripts/deploy_temporal_staging.sh

temporal-staging-sync-secrets: ## Sync Temporal SQL password secret from the RDS master secret
	@./scripts/sync_temporal_staging_secrets.sh

temporal-staging-verify: ## Verify Temporal staging deployments and the default namespace
	@./scripts/verify_temporal_staging.sh

external-secrets-install: ## Install the external-secrets operator
	@./scripts/install_external_secrets.sh

reloader-install: ## Install or upgrade Stakater Reloader
	@./scripts/install_reloader.sh

ingress-nginx-install-staging: ## Install or upgrade ingress-nginx
	@./scripts/install_ingress_nginx.sh

cert-manager-install: ## Install or upgrade cert-manager
	@./scripts/install_cert_manager.sh

cert-manager-apply-issuer: ## Apply a cert-manager ClusterIssuer manifest
	@ISSUER_FILE='$(ISSUER_FILE)' ./scripts/apply_cluster_issuer.sh

cloudflare-sync-dns-token: ## Create/update the cert-manager Cloudflare API token secret
	@./scripts/sync_cloudflare_dns_token.sh

build-staging-images: ## Build and push staging images to ECR
	@ENVIRONMENT=staging IMAGE_TAG='$(IMAGE_TAG)' API_IMAGE_REPOSITORY='$(API_IMAGE_REPOSITORY)' WEB_IMAGE_REPOSITORY='$(WEB_IMAGE_REPOSITORY)' AWS_REGION='$(AWS_REGION)' ./scripts/build_and_push_images.sh

test-integration: ## Run integration tests with real Postgres + Temporal
	@COMPOSE_FILE='$(COMPOSE_FILE)' TEST_DATABASE_URL='$(TEST_DATABASE_URL)' TEST_TEMPORAL_ADDRESS='$(TEST_TEMPORAL_ADDRESS)' TEST_TEMPORAL_NAMESPACE='$(TEST_TEMPORAL_NAMESPACE)' bash ./scripts/test_integration.sh

test-real-env-smoke: ## Run fast real-env smoke suite (Postgres + Temporal)
	@COMPOSE_FILE='$(COMPOSE_FILE)' TEST_DATABASE_URL='$(TEST_DATABASE_URL)' TEST_TEMPORAL_ADDRESS='$(TEST_TEMPORAL_ADDRESS)' TEST_TEMPORAL_NAMESPACE='$(TEST_TEMPORAL_NAMESPACE)' bash ./scripts/test_real_env_smoke.sh

test-staging-pricing-journey: ## Run live staging pricing configuration journey
	@bash ./scripts/run_staging_pricing_journey_with_minted_keys.sh

test-staging-access-invite-journey: ## Run live staging access and invite membership journey
	@bash ./scripts/run_staging_access_invite_journey.sh

test-staging-customer-onboarding-journey: ## Run live staging customer onboarding journey
	@bash ./scripts/run_staging_customer_onboarding_journey.sh

verify-staging-runtime: ## Verify staging runtime payment visibility + rate limiting
	@bash ./scripts/verify_staging_runtime.sh

verify-replay-smoke-staging: ## Create and verify a fresh live replay fixture in staging
	@bash ./scripts/verify_replay_smoke_staging.sh

test-staging-replay-smoke: verify-replay-smoke-staging ## Run live staging replay smoke

web-install: ## Install web dependencies
	@cd web && pnpm install

web-dev: ## Start web dev server
	@cd web && pnpm dev

web-lint: ## Lint web code
	@cd web && pnpm lint

web-build: ## Build web app
	@cd web && pnpm build

web-e2e: ## Run browser E2E tests
	@cd web && npx -y pnpm@10.30.0 exec playwright install --with-deps chromium && npx -y pnpm@10.30.0 build && npx -y pnpm@10.30.0 e2e

web-e2e-live: ## Run live staging browser smoke
	@PLAYWRIGHT_LIVE_BASE_URL='$(PLAYWRIGHT_LIVE_BASE_URL)' PLAYWRIGHT_LIVE_API_BASE_URL='$(PLAYWRIGHT_LIVE_API_BASE_URL)' PLAYWRIGHT_LIVE_PLATFORM_EMAIL='$(PLAYWRIGHT_LIVE_PLATFORM_EMAIL)' PLAYWRIGHT_LIVE_PLATFORM_PASSWORD='$(PLAYWRIGHT_LIVE_PLATFORM_PASSWORD)' PLAYWRIGHT_LIVE_WRITER_EMAIL='$(PLAYWRIGHT_LIVE_WRITER_EMAIL)' PLAYWRIGHT_LIVE_WRITER_PASSWORD='$(PLAYWRIGHT_LIVE_WRITER_PASSWORD)' PLAYWRIGHT_LIVE_READER_EMAIL='$(PLAYWRIGHT_LIVE_READER_EMAIL)' PLAYWRIGHT_LIVE_READER_PASSWORD='$(PLAYWRIGHT_LIVE_READER_PASSWORD)' PLAYWRIGHT_LIVE_PAYMENT_INVOICE_ID='$(PLAYWRIGHT_LIVE_PAYMENT_INVOICE_ID)' PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID='$(PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID)' PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID='$(PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID)' PLAYWRIGHT_LIVE_REPLAY_JOB_ID='$(PLAYWRIGHT_LIVE_REPLAY_JOB_ID)' PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID='$(PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID)' PLAYWRIGHT_LIVE_REPLAY_METER_ID='$(PLAYWRIGHT_LIVE_REPLAY_METER_ID)' bash ./scripts/run_live_browser_smoke.sh

test-browser-staging-smoke: web-e2e-live ## Run live staging browser smoke with real browser users

tf-fmt: ## Format Terraform code
	@terraform fmt -recursive $(TF_DIR)

tf-validate: ## Validate Terraform config
	@terraform -chdir=$(TF_DIR) init -backend=false
	@terraform -chdir=$(TF_DIR) validate

tf-plan-staging: ## Run Terraform plan for staging
	@ENVIRONMENT='staging' TF_DIR='$(TF_DIR)' ./scripts/terraform_plan.sh

tf-plan-prod: ## Run Terraform plan for prod
	@ENVIRONMENT='prod' TF_DIR='$(TF_DIR)' ./scripts/terraform_plan.sh

tf-apply-staging: ## Apply staging Terraform plan
	@ENVIRONMENT='staging' TF_DIR='$(TF_DIR)' ./scripts/terraform_apply.sh

tf-apply-prod: ## Apply prod Terraform plan
	@ENVIRONMENT='prod' TF_DIR='$(TF_DIR)' ./scripts/terraform_apply.sh

helm-lint: ## Lint Helm chart
	@helm lint $(HELM_CHART)

helm-template-staging: ## Render Helm staging manifests
	@helm template lago-alpha $(HELM_CHART) -f $(HELM_CHART)/environments/staging-values.yaml >/tmp/alpha-staging.yaml
	@echo "rendered /tmp/alpha-staging.yaml"

helm-template-prod: ## Render Helm prod manifests
	@helm template lago-alpha $(HELM_CHART) -f $(HELM_CHART)/environments/prod-values.yaml >/tmp/alpha-prod.yaml
	@echo "rendered /tmp/alpha-prod.yaml"

deploy-staging: ## Deploy Helm release to staging
	@ENVIRONMENT=staging RELEASE_NAME='$(RELEASE_NAME)' NAMESPACE='$(NAMESPACE)' IMAGE_TAG='$(IMAGE_TAG)' API_IMAGE_REPOSITORY='$(API_IMAGE_REPOSITORY)' WEB_IMAGE_REPOSITORY='$(WEB_IMAGE_REPOSITORY)' ./scripts/deploy_helm.sh

deploy-prod: ## Deploy Helm release to prod
	@ENVIRONMENT=prod RELEASE_NAME='$(RELEASE_NAME)' NAMESPACE='$(NAMESPACE)' IMAGE_TAG='$(IMAGE_TAG)' API_IMAGE_REPOSITORY='$(API_IMAGE_REPOSITORY)' WEB_IMAGE_REPOSITORY='$(WEB_IMAGE_REPOSITORY)' ./scripts/deploy_helm.sh

rollback-staging: ## Helm rollback in staging
	@ENVIRONMENT=staging RELEASE_NAME='$(RELEASE_NAME)' NAMESPACE='$(NAMESPACE)' REVISION='$(REVISION)' ./scripts/rollback_helm.sh

rollback-prod: ## Helm rollback in prod
	@ENVIRONMENT=prod RELEASE_NAME='$(RELEASE_NAME)' NAMESPACE='$(NAMESPACE)' REVISION='$(REVISION)' ./scripts/rollback_helm.sh

ci: fmt test ## CI convenience target
