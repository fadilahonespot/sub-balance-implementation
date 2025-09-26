# Makefile for Sub Balance Implementation

# Variables
APP_NAME=sub-balance-implementation
BINARY_NAME=sub-balance-app
DOCKER_IMAGE=sub-balance-app
VERSION=1.0.0

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_UNIX=$(BINARY_NAME)_unix

# Database parameters
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=sub_balance_db
DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

# Migration parameters
MIGRATE_CMD=migrate
MIGRATE_PATH=./migrations

.PHONY: all build clean test coverage deps run docker-build docker-run migrate-up migrate-down migrate-force migrate-version db-create db-drop db-reset help

# Default target
all: deps build

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/main.go

# Build for Linux
build-linux:
	@echo "Building for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v ./cmd/main.go

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Run the application
run:
	@echo "Running $(APP_NAME)..."
	$(GOCMD) run ./cmd/main.go

# Run with hot reload (requires air)
dev:
	@echo "Running in development mode..."
	air

# Database operations
db-create:
	@echo "Creating database..."
	createdb $(DB_NAME) || echo "Database already exists"

db-drop:
	@echo "Dropping database..."
	dropdb $(DB_NAME) || echo "Database does not exist"

db-reset: db-drop db-create migrate-up
	@echo "Database reset complete"

# Migration operations
migrate-up:
	@echo "Running migrations up..."
	$(MIGRATE_CMD) -path $(MIGRATE_PATH) -database "$(DB_URL)" up

migrate-down:
	@echo "Running migrations down..."
	$(MIGRATE_CMD) -path $(MIGRATE_PATH) -database "$(DB_URL)" down

migrate-force:
	@echo "Forcing migration version..."
	$(MIGRATE_CMD) -path $(MIGRATE_PATH) -database "$(DB_URL)" force $(VERSION)

migrate-version:
	@echo "Setting migration version..."
	$(MIGRATE_CMD) -path $(MIGRATE_PATH) -database "$(DB_URL)" version $(VERSION)

migrate-create:
	@echo "Creating new migration..."
	@read -p "Enter migration name: " name; \
	$(MIGRATE_CMD) create -ext sql -dir $(MIGRATE_PATH) $$name

# Docker operations
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 --name $(APP_NAME) $(DOCKER_IMAGE):latest

docker-stop:
	@echo "Stopping Docker container..."
	docker stop $(APP_NAME) || echo "Container not running"
	docker rm $(APP_NAME) || echo "Container not found"

docker-clean: docker-stop
	@echo "Cleaning Docker images..."
	docker rmi $(DOCKER_IMAGE):$(VERSION) || echo "Image not found"
	docker rmi $(DOCKER_IMAGE):latest || echo "Image not found"

# Linting and formatting
lint:
	@echo "Running linter..."
	golangci-lint run

fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

# Security scan
security:
	@echo "Running security scan..."
	gosec ./...

# Generate mocks
mocks:
	@echo "Generating mocks..."
	mockgen -source=internal/domain/account/account.go -destination=internal/domain/account/mocks/account_mock.go
	mockgen -source=internal/domain/account_balance_shard/account_balance_shard.go -destination=internal/domain/account_balance_shard/mocks/account_balance_shard_mock.go
	mockgen -source=internal/domain/transaction/transaction.go -destination=internal/domain/transaction/mocks/transaction_mock.go

# Setup development environment
setup: deps db-create migrate-up
	@echo "Development environment setup complete"

# Production deployment
deploy: clean build-linux docker-build
	@echo "Production deployment ready"

# Health check
health:
	@echo "Checking application health..."
	curl -f http://localhost:8080/health || echo "Application not responding"

# Load testing
load-test:
	@echo "Running load tests..."
	hey -n 1000 -c 10 http://localhost:8080/health

# Performance profiling
profile:
	@echo "Starting performance profiling..."
	$(GOCMD) run ./cmd/main.go &
	@sleep 5
	curl http://localhost:8080/debug/pprof/profile?seconds=30 > profile.out
	@echo "Profile saved to profile.out"

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  build-linux    - Build for Linux"
	@echo "  clean          - Clean build artifacts"
	@echo "  test           - Run tests"
	@echo "  coverage       - Run tests with coverage"
	@echo "  bench          - Run benchmarks"
	@echo "  deps           - Download dependencies"
	@echo "  run            - Run the application"
	@echo "  dev            - Run with hot reload"
	@echo "  db-create      - Create database"
	@echo "  db-drop        - Drop database"
	@echo "  db-reset       - Reset database"
	@echo "  migrate-up     - Run migrations up"
	@echo "  migrate-down   - Run migrations down"
	@echo "  migrate-force  - Force migration version"
	@echo "  migrate-create - Create new migration"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  docker-stop    - Stop Docker container"
	@echo "  docker-clean   - Clean Docker images"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  security       - Run security scan"
	@echo "  mocks          - Generate mocks"
	@echo "  setup          - Setup development environment"
	@echo "  deploy         - Prepare for production deployment"
	@echo "  health         - Check application health"
	@echo "  load-test      - Run load tests"
	@echo "  profile        - Start performance profiling"
	@echo "  help           - Show this help message"
