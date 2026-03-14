SHELL := /bin/bash

GO ?= go
COMPOSE_FILE ?= docker-compose.postgres.yml
DATABASE_URL ?= postgres://postgres:postgres@localhost:15432/lago_alpha?sslmode=disable
TEST_DATABASE_URL ?= postgres://postgres:postgres@localhost:15432/lago_alpha_test?sslmode=disable
TEST_TEMPORAL_ADDRESS ?= 127.0.0.1:17233
TEST_TEMPORAL_NAMESPACE ?= default
LAGO_REPO_PATH ?= ../lago
LAGO_COMPOSE_FILE ?= docker-compose.yml
TEST_LAGO_API_URL ?=
TEST_LAGO_API_KEY ?= lago_alpha_test_api_key
BOOTSTRAP_LAGO_FOR_TESTS ?= 1
CLEANUP_LAGO_ON_EXIT ?= 0
VERIFY_LAGO_BACKEND_FOR_TESTS ?= 0
LAGO_VERIFY_COMPOSE_FILE ?= docker-compose.dev.yml
CHECK_GITHUB ?= 0
RUN_GO_TESTS ?= 1
RUN_TERRAFORM_VALIDATE ?= 0
GITHUB_REPOSITORY ?=
TF_DIR ?= infra/terraform/aws
HELM_CHART ?= deploy/helm/lago-alpha
ENVIRONMENT ?= staging
RELEASE_NAME ?= lago-alpha
NAMESPACE ?= lago-alpha
IMAGE_TAG ?= $(shell git rev-parse HEAD)
API_IMAGE_REPOSITORY ?=
WEB_IMAGE_REPOSITORY ?=
REVISION ?=

.DEFAULT_GOAL := help

.PHONY: help fmt tidy test test-unit verify-governance preflight-release preflight-staging preflight-prod db-up db-down db-ps db-logs wait-db migrate migrate-up migrate-status migrate-verify run lago-up lago-down lago-ps lago-verify lago-staging-deploy lago-staging-sync-secrets lago-staging-verify lago-staging-checklist lago-staging-bootstrap-payments temporal-staging-deploy temporal-staging-sync-secrets temporal-staging-verify external-secrets-install ingress-nginx-install-staging cert-manager-install cert-manager-apply-issuer cloudflare-sync-dns-token build-staging-images test-integration test-real-env-smoke prepare-real-payment-fixture test-real-payment-e2e verify-staging-runtime verify-staging-acceptance backup-restore-drill rehearse-release-rollback web-install web-dev web-lint web-build web-e2e web-e2e-live tf-fmt tf-validate tf-plan tf-plan-staging tf-plan-prod tf-apply-staging tf-apply-prod helm-lint helm-template-staging helm-template-prod deploy-staging deploy-prod rollback-staging rollback-prod ci

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

verify-governance: ## Verify governance metadata (CODEOWNERS)
	@./scripts/verify_codeowners.sh

preflight-release: ## Run release preflight checks (ENVIRONMENT=staging|prod, optional CHECK_GITHUB=1)
	@ENVIRONMENT='$(ENVIRONMENT)' CHECK_GITHUB='$(CHECK_GITHUB)' RUN_GO_TESTS='$(RUN_GO_TESTS)' RUN_TERRAFORM_VALIDATE='$(RUN_TERRAFORM_VALIDATE)' GITHUB_REPOSITORY='$(GITHUB_REPOSITORY)' ./scripts/preflight_staging.sh

preflight-staging: ## Run release preflight checks for staging
	@ENVIRONMENT='staging' CHECK_GITHUB='$(CHECK_GITHUB)' RUN_GO_TESTS='$(RUN_GO_TESTS)' RUN_TERRAFORM_VALIDATE='$(RUN_TERRAFORM_VALIDATE)' GITHUB_REPOSITORY='$(GITHUB_REPOSITORY)' ./scripts/preflight_staging.sh

preflight-prod: ## Run release preflight checks for prod
	@ENVIRONMENT='prod' CHECK_GITHUB='$(CHECK_GITHUB)' RUN_GO_TESTS='$(RUN_GO_TESTS)' RUN_TERRAFORM_VALIDATE='$(RUN_TERRAFORM_VALIDATE)' GITHUB_REPOSITORY='$(GITHUB_REPOSITORY)' ./scripts/preflight_staging.sh

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

migrate-up: migrate ## Alias for migrate (apply pending migrations)

migrate-status: ## Show migration status (available/applied/pending/unknown)
	@DATABASE_URL='$(DATABASE_URL)' $(GO) run ./cmd/migrate status

migrate-verify: ## Verify no pending or unknown applied migrations remain
	@DATABASE_URL='$(DATABASE_URL)' $(GO) run ./cmd/migrate verify

run: ## Start API server
	@DATABASE_URL='$(DATABASE_URL)' $(GO) run ./cmd/server

lago-up: ## Start Lago services and provision deterministic API key for tests
	@LAGO_REPO_PATH='$(LAGO_REPO_PATH)' LAGO_COMPOSE_FILE='$(LAGO_COMPOSE_FILE)' TEST_LAGO_API_URL='$(TEST_LAGO_API_URL)' TEST_LAGO_API_KEY='$(TEST_LAGO_API_KEY)' bash ./scripts/bootstrap_lago.sh

lago-down: ## Stop Lago compose stack
	@cd '$(LAGO_REPO_PATH)' && docker compose -f '$(LAGO_COMPOSE_FILE)' down

lago-ps: ## Show Lago compose service status
	@cd '$(LAGO_REPO_PATH)' && docker compose -f '$(LAGO_COMPOSE_FILE)' ps

lago-verify: ## Run Lago replay/correctness verification suites
	@cd '$(LAGO_REPO_PATH)' && ./scripts/verify_e2e.sh

lago-staging-deploy: ## Deploy Lago staging into the current cluster (expects deploy/lago/environments/staging-values.yaml)
	@./scripts/deploy_lago_staging.sh

lago-staging-sync-secrets: ## Sync Lago staging runtime secrets from AWS Secrets Manager into Kubernetes
	@./scripts/sync_lago_staging_secrets.sh

lago-staging-verify: ## Verify Lago staging namespace, services, and optional API reachability
	@./scripts/verify_lago_staging.sh

lago-staging-checklist: ## Print first-time manual Lago bootstrap steps
	@./scripts/print_lago_bootstrap_checklist.sh

lago-staging-bootstrap-payments: ## Bootstrap Lago Stripe/webhook/test-customer payment fixtures in staging
	@./scripts/bootstrap_lago_stripe_staging.sh

temporal-staging-deploy: ## Deploy Temporal staging into the current cluster (official Helm chart, internal only)
	@./scripts/deploy_temporal_staging.sh

temporal-staging-sync-secrets: ## Sync Temporal SQL password secret from AWS RDS master secret into Kubernetes
	@./scripts/sync_temporal_staging_secrets.sh

temporal-staging-verify: ## Verify Temporal staging deployments and the default namespace
	@./scripts/verify_temporal_staging.sh

external-secrets-install: ## Install the external-secrets operator with staging IRSA wiring
	@./scripts/install_external_secrets.sh

ingress-nginx-install-staging: ## Install or upgrade ingress-nginx with tracked staging values
	@./scripts/install_ingress_nginx.sh

cert-manager-install: ## Install or upgrade cert-manager in the current cluster
	@./scripts/install_cert_manager.sh

cert-manager-apply-issuer: ## Apply a cert-manager ClusterIssuer manifest (ISSUER_FILE=...)
	@ISSUER_FILE='$(ISSUER_FILE)' ./scripts/apply_cluster_issuer.sh

cloudflare-sync-dns-token: ## Create/update the cert-manager Cloudflare API token secret (requires CLOUDFLARE_API_TOKEN)
	@./scripts/sync_cloudflare_dns_token.sh

build-staging-images: ## Build and push linux/amd64 staging images to ECR (requires IMAGE_TAG/API_IMAGE_REPOSITORY/WEB_IMAGE_REPOSITORY)
	@ENVIRONMENT=staging IMAGE_TAG='$(IMAGE_TAG)' API_IMAGE_REPOSITORY='$(API_IMAGE_REPOSITORY)' WEB_IMAGE_REPOSITORY='$(WEB_IMAGE_REPOSITORY)' AWS_REGION='$(AWS_REGION)' ./scripts/build_and_push_images.sh

test-integration: ## Run integration tests with real Postgres + real Lago
	@COMPOSE_FILE='$(COMPOSE_FILE)' TEST_DATABASE_URL='$(TEST_DATABASE_URL)' TEST_TEMPORAL_ADDRESS='$(TEST_TEMPORAL_ADDRESS)' TEST_TEMPORAL_NAMESPACE='$(TEST_TEMPORAL_NAMESPACE)' TEST_LAGO_API_URL='$(TEST_LAGO_API_URL)' TEST_LAGO_API_KEY='$(TEST_LAGO_API_KEY)' BOOTSTRAP_LAGO_FOR_TESTS='$(BOOTSTRAP_LAGO_FOR_TESTS)' LAGO_REPO_PATH='$(LAGO_REPO_PATH)' LAGO_COMPOSE_FILE='$(LAGO_COMPOSE_FILE)' CLEANUP_LAGO_ON_EXIT='$(CLEANUP_LAGO_ON_EXIT)' VERIFY_LAGO_BACKEND_FOR_TESTS='$(VERIFY_LAGO_BACKEND_FOR_TESTS)' LAGO_VERIFY_COMPOSE_FILE='$(LAGO_VERIFY_COMPOSE_FILE)' bash ./scripts/test_integration.sh

test-real-env-smoke: ## Run fast real-env smoke suite (Postgres + Temporal + Lago)
	@COMPOSE_FILE='$(COMPOSE_FILE)' TEST_DATABASE_URL='$(TEST_DATABASE_URL)' TEST_TEMPORAL_ADDRESS='$(TEST_TEMPORAL_ADDRESS)' TEST_TEMPORAL_NAMESPACE='$(TEST_TEMPORAL_NAMESPACE)' TEST_LAGO_API_URL='$(TEST_LAGO_API_URL)' TEST_LAGO_API_KEY='$(TEST_LAGO_API_KEY)' BOOTSTRAP_LAGO_FOR_TESTS='$(BOOTSTRAP_LAGO_FOR_TESTS)' LAGO_REPO_PATH='$(LAGO_REPO_PATH)' LAGO_COMPOSE_FILE='$(LAGO_COMPOSE_FILE)' CLEANUP_LAGO_ON_EXIT='$(CLEANUP_LAGO_ON_EXIT)' VERIFY_LAGO_BACKEND_FOR_TESTS='$(VERIFY_LAGO_BACKEND_FOR_TESTS)' LAGO_VERIFY_COMPOSE_FILE='$(LAGO_VERIFY_COMPOSE_FILE)' bash ./scripts/test_real_env_smoke.sh

prepare-real-payment-fixture: ## Prepare collectible Lago invoice fixture (requires LAGO_API_URL/LAGO_API_KEY/CUSTOMER_EXTERNAL_ID)
	@bash ./scripts/prepare_real_payment_invoice_fixture.sh

test-real-payment-e2e: ## Run manual real payment collection E2E (requires staging/prod credentials + invoice id)
	@bash ./scripts/test_real_payment_e2e.sh

verify-staging-runtime: ## Verify staging runtime payment visibility + rate limiting (requires ALPHA_API_BASE_URL/ALPHA_READER_API_KEY)
	@bash ./scripts/verify_staging_runtime.sh

verify-staging-acceptance: ## Run staging runtime verify + success/failure payment E2E (requires staging URLs/keys/invoice ids)
	@bash ./scripts/verify_staging_acceptance.sh

backup-restore-drill: ## Run RDS backup+restore drill (requires AWS env vars and CONFIRM_BACKUP_RESTORE=YES_I_UNDERSTAND)
	@bash ./scripts/rds_backup_restore_drill.sh

rehearse-release-rollback: ## Run deploy -> rollback -> redeploy rehearsal (requires kubectl/helm context + image vars)
	@bash ./scripts/rehearse_release_rollback.sh

web-e2e: ## Run browser E2E tests for control-plane UI
	@cd web && npx -y pnpm@10.30.0 exec playwright install --with-deps chromium && npx -y pnpm@10.30.0 build && npx -y pnpm@10.30.0 e2e

web-e2e-live: ## Run live staging browser smoke for payment-ops and optional invoice explainability (requires PLAYWRIGHT_LIVE_BASE_URL/PLAYWRIGHT_LIVE_API_BASE_URL/PLAYWRIGHT_LIVE_WRITER_API_KEY; optional PLAYWRIGHT_LIVE_READER_API_KEY/PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID)
	@cd web && npx -y pnpm@10.30.0 exec playwright install --with-deps chromium && PLAYWRIGHT_LIVE_BASE_URL='$(PLAYWRIGHT_LIVE_BASE_URL)' PLAYWRIGHT_LIVE_API_BASE_URL='$(PLAYWRIGHT_LIVE_API_BASE_URL)' PLAYWRIGHT_LIVE_API_KEY='$(PLAYWRIGHT_LIVE_API_KEY)' PLAYWRIGHT_LIVE_WRITER_API_KEY='$(PLAYWRIGHT_LIVE_WRITER_API_KEY)' PLAYWRIGHT_LIVE_READER_API_KEY='$(PLAYWRIGHT_LIVE_READER_API_KEY)' PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID='$(PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID)' npx -y pnpm@10.30.0 exec playwright test tests/e2e/payment-operations-live.spec.ts tests/e2e/invoice-explainability-live.spec.ts --workers=1

tf-fmt: ## Format Terraform code
	@terraform fmt -recursive $(TF_DIR)

tf-validate: ## Validate Terraform config (without backend)
	@terraform -chdir=$(TF_DIR) init -backend=false
	@terraform -chdir=$(TF_DIR) validate

tf-plan: ## Run Terraform plan (requires configured backend/vars)
	@ENVIRONMENT='$(ENVIRONMENT)' TF_DIR='$(TF_DIR)' ./scripts/terraform_plan.sh

tf-plan-staging: ## Run Terraform plan for staging using env/backends files
	@ENVIRONMENT='staging' TF_DIR='$(TF_DIR)' ./scripts/terraform_plan.sh

tf-plan-prod: ## Run Terraform plan for prod using env/backends files
	@ENVIRONMENT='prod' TF_DIR='$(TF_DIR)' ./scripts/terraform_plan.sh

tf-apply-staging: ## Apply previously created staging plan
	@ENVIRONMENT='staging' TF_DIR='$(TF_DIR)' ./scripts/terraform_apply.sh

tf-apply-prod: ## Apply previously created prod plan
	@ENVIRONMENT='prod' TF_DIR='$(TF_DIR)' ./scripts/terraform_apply.sh

helm-lint: ## Lint Helm chart
	@helm lint $(HELM_CHART)

helm-template-staging: ## Render Helm staging manifests
	@helm template lago-alpha $(HELM_CHART) -f $(HELM_CHART)/environments/staging-values.yaml >/tmp/lago-alpha-staging.yaml
	@echo "rendered /tmp/lago-alpha-staging.yaml"

helm-template-prod: ## Render Helm prod manifests
	@helm template lago-alpha $(HELM_CHART) -f $(HELM_CHART)/environments/prod-values.yaml >/tmp/lago-alpha-prod.yaml
	@echo "rendered /tmp/lago-alpha-prod.yaml"

deploy-staging: ## Deploy Helm release to staging (requires kubectl context + image vars)
	@ENVIRONMENT=staging RELEASE_NAME='$(RELEASE_NAME)' NAMESPACE='$(NAMESPACE)' IMAGE_TAG='$(IMAGE_TAG)' API_IMAGE_REPOSITORY='$(API_IMAGE_REPOSITORY)' WEB_IMAGE_REPOSITORY='$(WEB_IMAGE_REPOSITORY)' ./scripts/deploy_helm.sh

deploy-prod: ## Deploy Helm release to prod (requires kubectl context + image vars)
	@ENVIRONMENT=prod RELEASE_NAME='$(RELEASE_NAME)' NAMESPACE='$(NAMESPACE)' IMAGE_TAG='$(IMAGE_TAG)' API_IMAGE_REPOSITORY='$(API_IMAGE_REPOSITORY)' WEB_IMAGE_REPOSITORY='$(WEB_IMAGE_REPOSITORY)' ./scripts/deploy_helm.sh

rollback-staging: ## Helm rollback in staging (requires REVISION and kubectl context)
	@ENVIRONMENT=staging RELEASE_NAME='$(RELEASE_NAME)' NAMESPACE='$(NAMESPACE)' REVISION='$(REVISION)' ./scripts/rollback_helm.sh

rollback-prod: ## Helm rollback in prod (requires REVISION and kubectl context)
	@ENVIRONMENT=prod RELEASE_NAME='$(RELEASE_NAME)' NAMESPACE='$(NAMESPACE)' REVISION='$(REVISION)' ./scripts/rollback_helm.sh

ci: fmt test ## CI convenience target (format + full tests)
