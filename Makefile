# FinWatch developer commands.
# Backend lives in apps/api (Go); frontend in apps/web (Node/npm); the full
# local stack runs via Docker Compose.

API_DIR := apps/api
WEB_DIR := apps/web
COMPOSE := docker compose

.DEFAULT_GOAL := help
N ?= 50

.PHONY: help bootstrap dev stop lint test build verify sqlc seed \
        api-lint api-test api-build web-lint web-typecheck web-test web-build compose-config

help: ## List available targets
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

bootstrap: ## Install backend and frontend dependencies
	cd $(API_DIR) && go mod download
	cd $(WEB_DIR) && npm install

dev: ## Start the full stack (db, api, web) via Docker Compose
	$(COMPOSE) up --build -d
	@echo "API  -> http://localhost:8080/health/live"
	@echo "Web  -> http://localhost:8081"

stop: ## Stop the local stack
	$(COMPOSE) down

lint: api-lint web-lint ## Lint backend and frontend

test: api-test web-test ## Run backend and frontend tests

build: api-build web-build ## Build backend and frontend

# verify is the full pre-commit / CI gate.
verify: api-lint api-test api-build web-lint web-typecheck web-test web-build ## Run every quality gate

api-lint:
	cd $(API_DIR) && test -z "$$(gofmt -l .)" || (echo "gofmt: files need formatting" && gofmt -l . && exit 1)
	cd $(API_DIR) && go vet ./...

api-test:
	cd $(API_DIR) && go test -p 1 ./...

api-build:
	cd $(API_DIR) && go build ./...

web-lint:
	cd $(WEB_DIR) && npm run lint

web-typecheck:
	cd $(WEB_DIR) && npm run typecheck

web-test:
	cd $(WEB_DIR) && npm run test

web-build:
	cd $(WEB_DIR) && npm run build

compose-config: ## Validate the Docker Compose file
	$(COMPOSE) config -q && echo "docker-compose.yml is valid"

sqlc: ## Regenerate type-safe DB code from SQL (requires sqlc on PATH)
	cd $(API_DIR) && sqlc generate

seed: ## Ingest N synthetic transactions (make seed N=100); needs DATABASE_URL
	cd $(API_DIR) && go run ./cmd/api seed -n $(N)
