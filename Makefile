.PHONY: dev build run swag test test-integration test-e2e loadtest loadtest-smoke loadtest-docker up seed seed-local down lint lint-fix help

# Prefer `docker compose` (V2); fall back to `docker-compose` (V1).
DOCKER_COMPOSE ?= $(shell docker compose version >/dev/null 2>&1 && echo docker compose || echo docker-compose)

# Packages for default test/coverage (exclude ./e2e — needs Docker; use test-e2e).
TEST_PKGS ?= $(shell go list ./... | grep -v '/e2e$$' | grep -v '/docs/swagger$$')
GOTESTSUM ?= go tool gotestsum

up:
	@echo "Starting Docker Compose stack..."
	$(DOCKER_COMPOSE) up -d --build

down:
	@echo "Stopping Docker Compose stack..."
	$(DOCKER_COMPOSE) down

# Меню заливает partner-demo при старте; restart повторяет PUT /partner/menu.
seed:
	@echo "Re-sync demo menu (restart partner-demo)..."
	$(DOCKER_COMPOSE) restart partner-demo

# Seed using local partner-demo; uses .env (DATABASE_URL / KITCHEN_BASE_URL=localhost).
seed-local:
	@echo "Seeding catalog: start partner-demo (it PUTs menu to kitchen-api)..."
	@echo "Need kitchen-api on :8080, then: go run ./cmd/partner-demo"
	go run ./cmd/partner-demo

dev:
	@echo "Starting kitchen-api (needs Postgres + Redis, e.g. make up)..."
	@echo "Swagger UI: http://localhost:8080/swagger/index.html"
	go run ./cmd/kitchen-api

build:
	@echo "Building Go applications..."
	@mkdir -p bin
	go build -o bin/kitchen-api ./cmd/kitchen-api
	go build -o bin/partner-demo ./cmd/partner-demo
	@echo "Build complete: bin/kitchen-api bin/partner-demo"

run:
	@echo "Running kitchen-api..."
	@echo "Swagger UI: http://localhost:8080/swagger/index.html"
	go run ./cmd/kitchen-api

swag:
	@echo "Generating Swagger from godoc (swaggo) -> docs/swagger ..."
	go tool swag init -g cmd/kitchen-api/main.go -o docs/swagger --parseInternal --exclude e2e

test:
	@echo "Running tests with gotestsum (coverage -> coverage.out)..."
	$(GOTESTSUM) --format testname -- -count=1 -cover -coverprofile=coverage.out -covermode=atomic $(TEST_PKGS)
	@echo ""
	@echo "Per-file summary (coverage.out):"
	@go tool cover -func=coverage.out

# Integration tests (testcontainers Postgres; requires Docker).
test-integration:
	@echo "Running integration-tagged tests (Docker required)..."
	$(GOTESTSUM) --format testname -- -count=1 -timeout 8m -tags=integration -cover -coverprofile=coverage-integration.out -covermode=atomic $(TEST_PKGS)
	@echo ""
	@echo "Per-file summary (integration, coverage-integration.out):"
	@go tool cover -func=coverage-integration.out

test-e2e:
	@echo "E2E tests (Docker: testcontainers Postgres + in-process API; or set E2E_BASE_URL for an external server)..."
	$(GOTESTSUM) --format testname -- -count=1 -timeout 8m -v ./e2e/

# Load test: k6 against GET /api/v1/restaurants. Stack should be up (make up).
BASE_URL ?= http://127.0.0.1:8080
LOADTEST_RATE ?= 50
LOADTEST_DURATION ?= 90s

loadtest:
	@command -v k6 >/dev/null 2>&1 || { echo "Install k6: https://grafana.com/docs/k6/latest/set-up/install-k6/"; exit 1; }
	k6 run loadtest/k6/catalog.js -e BASE_URL=$(BASE_URL) -e LOADTEST_RATE=$(LOADTEST_RATE) -e LOADTEST_DURATION=$(LOADTEST_DURATION)

loadtest-smoke:
	@command -v k6 >/dev/null 2>&1 || { echo "Install k6: https://grafana.com/docs/k6/latest/set-up/install-k6/"; exit 1; }
	k6 run loadtest/k6/catalog.js -e BASE_URL=$(BASE_URL) -e LOADTEST_PROFILE=smoke

# k6 via Docker (no local k6). Linux: set BASE_URL to host IP if host.docker.internal is unavailable.
loadtest-docker:
	docker run --rm -i --add-host=host.docker.internal:host-gateway \
		-e BASE_URL=http://host.docker.internal:8080 \
		-e LOADTEST_RATE=$(LOADTEST_RATE) \
		-e LOADTEST_DURATION=$(LOADTEST_DURATION) \
		grafana/k6:0.57.0 run - < loadtest/k6/catalog.js

lint:
	@echo "Running linter..."
	golangci-lint run ./...

lint-fix:
	@echo "Running linter with autofix..."
	golangci-lint run --fix ./...

help:
	@echo "Available commands:"
	@echo "  up             - Start project with dependencies (docker compose up -d --build)"
	@echo "  down           - Stop Docker Compose stack"
	@echo "  seed           - Re-upload demo menu (restart partner-demo)"
	@echo "  seed-local     - Run partner-demo locally (PUTs menu to kitchen-api)"
	@echo "  dev            - go run ./cmd/kitchen-api"
	@echo "  build          - Build kitchen-api and partner-demo into ./bin/"
	@echo "  run            - go run ./cmd/kitchen-api"
	@echo "  swag           - Generate docs/swagger from godoc (swaggo); UI at /swagger/index.html"
	@echo "  test           - gotestsum (go tool from go.mod) + coverage -> coverage.out"
	@echo "  test-integration - Integration tests (-tags=integration) + coverage -> coverage-integration.out"
	@echo "  test-e2e       - HTTP E2E (testcontainers Postgres by default; optional E2E_BASE_URL=... for external API)"
	@echo "  loadtest       - k6 GET /api/v1/restaurants (BASE_URL, LOADTEST_RATE, LOADTEST_DURATION)"
	@echo "  loadtest-smoke - Short k6 sanity run (LOADTEST_PROFILE=smoke)"
	@echo "  loadtest-docker - Same as loadtest using grafana/k6 image"
	@echo "  lint           - Run linter"
	@echo "  lint-fix       - Run linter with autofix"
	@echo "  help           - Show this help"
