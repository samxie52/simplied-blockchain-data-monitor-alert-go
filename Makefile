# Makefile for Blockchain Monitor Alert System

# Variables
APP_NAME=blockchain-monitor
VERSION=v1.0.0
GO_VERSION=1.21
DOCKER_REGISTRY=your-registry
DOCKER_IMAGE=$(DOCKER_REGISTRY)/$(APP_NAME)

# Go related variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOFMT=gofmt
GOVET=$(GOCMD) vet
GOLINT=golangci-lint

# Build targets
SERVER_BINARY=bin/server
WORKER_BINARY=bin/worker
MIGRATOR_BINARY=bin/migrator

# Default target
.PHONY: all
all: clean deps test build

# Help target
.PHONY: help
help: ## Display this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development targets
.PHONY: deps
deps: ## Install dependencies
	$(GOGET) -v ./...
	$(GOCMD) mod tidy
	$(GOCMD) mod download

.PHONY: deps-update
deps-update: ## Update dependencies
	$(GOCMD) get -u ./...
	$(GOCMD) mod tidy

# Build targets
.PHONY: build
build: build-server build-worker build-migrator ## Build all binaries

.PHONY: build-server
build-server: ## Build server binary
	mkdir -p bin
	$(GOBUILD) -o $(SERVER_BINARY) -v ./cmd/server

.PHONY: build-worker
build-worker: ## Build worker binary
	mkdir -p bin
	$(GOBUILD) -o $(WORKER_BINARY) -v ./cmd/worker

.PHONY: build-migrator
build-migrator: ## Build migrator binary
	mkdir -p bin
	$(GOBUILD) -o $(MIGRATOR_BINARY) -v ./cmd/migrator

.PHONY: build-linux
build-linux: ## Build for Linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(SERVER_BINARY)-linux -v ./cmd/server
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(WORKER_BINARY)-linux -v ./cmd/worker
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(MIGRATOR_BINARY)-linux -v ./cmd/migrator

# Test targets
.PHONY: test
test: ## Run tests
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

.PHONY: test-unit
test-unit: ## Run unit tests
	$(GOTEST) -v -race ./tests/unit/...

.PHONY: test-integration
test-integration: ## Run integration tests
	$(GOTEST) -v -race ./tests/integration/...

.PHONY: test-coverage
test-coverage: test ## Generate test coverage report
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Code quality targets
.PHONY: fmt
fmt: ## Format code
	$(GOFMT) -s -w .

.PHONY: vet
vet: ## Run go vet
	$(GOVET) ./...

.PHONY: lint
lint: ## Run linter
	$(GOLINT) run ./...

.PHONY: check
check: fmt vet lint test ## Run all checks

# Development server targets
.PHONY: dev
dev: ## Run development server
	air -c .air.toml

.PHONY: run-server
run-server: build-server ## Run server
	./$(SERVER_BINARY)

.PHONY: run-worker
run-worker: build-worker ## Run worker
	./$(WORKER_BINARY)

# Database targets
.PHONY: migrate
migrate: build-migrator ## Run database migrations
	./$(MIGRATOR_BINARY) up

.PHONY: migrate-down
migrate-down: build-migrator ## Rollback database migrations
	./$(MIGRATOR_BINARY) down

.PHONY: migrate-reset
migrate-reset: build-migrator ## Reset database
	./$(MIGRATOR_BINARY) reset

# Docker targets
.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE):$(VERSION) -f deployments/docker/Dockerfile .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

.PHONY: docker-push
docker-push: ## Push Docker image
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

.PHONY: docker-up
docker-up: ## Start services with docker-compose
	docker-compose -f deployments/docker/docker-compose.yml up -d

.PHONY: docker-down
docker-down: ## Stop services with docker-compose
	docker-compose -f deployments/docker/docker-compose.yml down

.PHONY: docker-logs
docker-logs: ## View docker-compose logs
	docker-compose -f deployments/docker/docker-compose.yml logs -f

# Cleanup targets
.PHONY: clean
clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.out coverage.html

.PHONY: clean-docker
clean-docker: ## Clean Docker images
	docker rmi $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest || true
	docker system prune -f

# Documentation targets
.PHONY: docs
docs: ## Generate documentation
	@echo "Generating API documentation..."
	swag init -g cmd/server/main.go -o docs/swagger

# Installation targets
.PHONY: install-tools
install-tools: ## Install development tools
	go install github.com/cosmtrek/air@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Git targets
.PHONY: git-hooks
git-hooks: ## Install git hooks
	cp scripts/pre-commit .git/hooks/
	chmod +x .git/hooks/pre-commit

# Environment setup
.PHONY: setup
setup: install-tools deps git-hooks ## Setup development environment
	cp .env.example .env
	@echo "Development environment setup complete!"
	@echo "Please edit .env file with your configuration"

# Release targets
.PHONY: release
release: clean check build docker-build ## Prepare release
	@echo "Release $(VERSION) ready"

# Show project info
.PHONY: info
info: ## Show project information
	@echo "Project: $(APP_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Go Version: $(GO_VERSION)"
	@echo "Docker Image: $(DOCKER_IMAGE)"