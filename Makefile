SHELL := /bin/bash

GO ?= go
COMPOSE_FILE ?= docker-compose.postgres.yml
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/lago_alpha?sslmode=disable
TEST_DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/lago_alpha_test?sslmode=disable

.DEFAULT_GOAL := help

.PHONY: help fmt tidy test test-unit db-up db-down db-ps db-logs wait-db migrate migrate-up migrate-status migrate-verify run test-integration ci

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

test-integration: ## Run integration tests with real Postgres
	@COMPOSE_FILE='$(COMPOSE_FILE)' TEST_DATABASE_URL='$(TEST_DATABASE_URL)' ./scripts/test_integration.sh

ci: fmt test ## CI convenience target (format + full tests)
