.PHONY: help dev test test-backend test-frontend lint build clean migrate docker-up docker-down ci

# Default target
help:
	@echo "AutoDreamApplier — Development Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make dev          Start full local stack (Docker Compose)"
	@echo "  make test         Run all tests (backend + frontend)"
	@echo "  make test-backend Run Go tests only"
	@echo "  make test-frontend Run frontend build + lint"
	@echo "  make lint         Run go vet + golangci-lint"
	@echo "  make build        Build all Go binaries"
	@echo "  make migrate      Run database migrations"
	@echo "  make docker-up    Start infrastructure services"
	@echo "  make docker-down  Stop and remove containers"
	@echo "  make ci           Run all CI checks locally (mirrors GitHub Actions)"

# ── Development ────────────────────────────────────────────────────────────────

dev:
	docker compose up --build

docker-up:
	docker compose up -d postgres redis minio

docker-down:
	docker compose down -v

# ── Testing ────────────────────────────────────────────────────────────────────

test: test-backend test-frontend

test-backend:
	@echo "→ Running Go tests..."
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" \
	APP_ENV=test \
	DEV_JWT_SECRET=ci-test-secret-32byteslong12345 \
	ENCRYPTION_KEY=0000000000000000000000000000000000000000000000000000000000000000 \
	go test -race -timeout 120s ./...

test-frontend:
	@echo "→ Running frontend checks..."
	cd frontend && npm ci && npm run build && npm run lint

# ── Code Quality ───────────────────────────────────────────────────────────────

lint:
	go vet ./...
	@which golangci-lint > /dev/null && golangci-lint run || echo "golangci-lint not installed — skipping"

# ── Build ──────────────────────────────────────────────────────────────────────

build:
	go build -v ./cmd/api-gateway/...
	go build -v ./cmd/apply-engine/...
	go build -v ./cmd/job-discovery/...

# ── Database ───────────────────────────────────────────────────────────────────

migrate:
	go run ./cmd/db-migrator/...

# ── CI (mirrors GitHub Actions locally) ───────────────────────────────────────

ci: lint test-backend test-frontend build
	@echo "✅ All CI checks passed"
