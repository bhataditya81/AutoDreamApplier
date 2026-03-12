.PHONY: all build run test clean migrate docker-up docker-down lint dev

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOMOD=$(GOCMD) mod
BINARY_DIR=bin

# Services
SERVICES=api-gateway job-discovery job-matcher apply-engine browser-pool

all: build

## dev: Spin up the full local dev stack (Docker + migrations + Next.js)
## dev: On Windows use: powershell -ExecutionPolicy Bypass -File scripts\dev.ps1
dev:
	@chmod +x scripts/dev.sh
	@./scripts/dev.sh

## dev-no-build: Start dev stack without rebuilding Docker images
dev-no-build:
	@chmod +x scripts/dev.sh
	@./scripts/dev.sh --no-build

## dev-backend: Start backend only (no Next.js)
dev-backend:
	@chmod +x scripts/dev.sh
	@./scripts/dev.sh --no-frontend

## build: Build all Go services
build:
	@echo "Building all services..."
	@mkdir -p $(BINARY_DIR)
	@for service in $(SERVICES); do \
		echo "  Building $$service..."; \
		$(GOBUILD) -o $(BINARY_DIR)/$$service ./cmd/$$service/; \
	done
	@echo "Build complete."

## build-service: Build a single service (usage: make build-service SERVICE=api-gateway)
build-service:
	@echo "Building $(SERVICE)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(SERVICE) ./cmd/$(SERVICE)/
	@echo "Build complete."

## run-api: Run the API gateway locally
run-api:
	$(GOCMD) run ./cmd/api-gateway/

## test: Run all Go tests
test:
	$(GOTEST) -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## lint: Run linter
lint:
	golangci-lint run ./...

## vet: Run go vet
vet:
	$(GOVET) ./...

## tidy: Tidy go modules
tidy:
	$(GOMOD) tidy

## clean: Clean build artifacts
clean:
	@rm -rf $(BINARY_DIR)
	@rm -f coverage.out coverage.html
	@echo "Cleaned."

## migrate-up: Run database migrations up
migrate-up:
	@./scripts/migrate.sh up

## migrate-down: Run database migrations down
migrate-down:
	@./scripts/migrate.sh down

## migrate-create: Create a new migration (usage: make migrate-create NAME=add_users_table)
migrate-create:
	migrate create -ext sql -dir migrations -seq $(NAME)

## docker-up: Start all services with Docker Compose
docker-up:
	docker-compose up -d
	@echo "Services started. API available at http://localhost:8080"

## docker-down: Stop all Docker Compose services
docker-down:
	docker-compose down

## docker-build: Build Docker images
docker-build:
	docker-compose build

## docker-logs: View Docker Compose logs
docker-logs:
	docker-compose logs -f

## seed: Seed development database
seed:
	@./scripts/seed.sh

## ai-service: Run the Python AI service locally
ai-service:
	cd ai-service && pip install -r requirements.txt && uvicorn app.main:app --reload --port 8081

## help: Show this help
help:
	@echo "AutoDreamApplier - Available commands:"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'
