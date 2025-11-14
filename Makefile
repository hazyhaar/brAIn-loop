.PHONY: help build test lint clean ci install dev coverage benchmark security

# Variables
BINARY_NAME=brainloop
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags="-s -w"
COVERAGE_FILE=coverage.out

# Colors
GREEN=\033[0;32m
YELLOW=\033[0;33m
NC=\033[0m # No Color

help: ## Show this help
	@echo "$(GREEN)Available targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'

build: ## Build binary
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) main.go
	@echo "$(GREEN)✓ Build complete$(NC)"

install: ## Install binary to $GOPATH/bin
	@echo "$(GREEN)Installing $(BINARY_NAME)...$(NC)"
	$(GO) install $(LDFLAGS) .
	@echo "$(GREEN)✓ Installed to $(GOPATH)/bin/$(BINARY_NAME)$(NC)"

test: ## Run tests
	@echo "$(GREEN)Running tests...$(NC)"
	$(GO) test $(GOFLAGS) -race ./...
	@echo "$(GREEN)✓ Tests passed$(NC)"

test-verbose: ## Run tests with verbose output
	@echo "$(GREEN)Running tests (verbose)...$(NC)"
	$(GO) test -v -race ./...

test-short: ## Run short tests only
	@echo "$(GREEN)Running short tests...$(NC)"
	$(GO) test -short ./...

coverage: ## Generate coverage report
	@echo "$(GREEN)Generating coverage report...$(NC)"
	$(GO) test -coverprofile=$(COVERAGE_FILE) ./...
	$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	$(GO) tool cover -func=$(COVERAGE_FILE) | grep total
	@echo "$(GREEN)✓ Coverage report: coverage.html$(NC)"

benchmark: ## Run benchmarks
	@echo "$(GREEN)Running benchmarks...$(NC)"
	$(GO) test -bench=. -benchmem ./...

lint: ## Run linter
	@echo "$(GREEN)Running linter...$(NC)"
	@which golangci-lint > /dev/null || (echo "$(YELLOW)golangci-lint not found. Installing...$(NC)" && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run --timeout=5m
	@echo "$(GREEN)✓ Linting passed$(NC)"

lint-fix: ## Run linter with auto-fix
	@echo "$(GREEN)Running linter with auto-fix...$(NC)"
	golangci-lint run --fix --timeout=5m

fmt: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GO) fmt ./...
	@which goimports > /dev/null && goimports -w . || true
	@echo "$(GREEN)✓ Code formatted$(NC)"

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)✓ Vet passed$(NC)"

security: ## Run security scanner
	@echo "$(GREEN)Running security scanner...$(NC)"
	@which gosec > /dev/null || (echo "$(YELLOW)gosec not found. Installing...$(NC)" && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec -quiet ./...
	@echo "$(GREEN)✓ Security scan passed$(NC)"

clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning...$(NC)"
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE_FILE) coverage.html
	rm -f *.db *.db-shm *.db-wal
	rm -f brainloop.lock
	$(GO) clean
	@echo "$(GREEN)✓ Cleaned$(NC)"

dev: ## Install development tools
	@echo "$(GREEN)Installing development tools...$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "$(GREEN)✓ Development tools installed$(NC)"

ci: fmt vet lint test security build ## Run all CI checks locally
	@echo "$(GREEN)✓ All CI checks passed$(NC)"

run: build ## Build and run
	@echo "$(GREEN)Starting $(BINARY_NAME)...$(NC)"
	./$(BINARY_NAME)

docker-build: ## Build Docker image
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t $(BINARY_NAME):latest .

docker-run: docker-build ## Build and run Docker container
	@echo "$(GREEN)Running Docker container...$(NC)"
	docker run --rm -it $(BINARY_NAME):latest

deps: ## Download dependencies
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	$(GO) mod download
	@echo "$(GREEN)✓ Dependencies downloaded$(NC)"

deps-update: ## Update dependencies
	@echo "$(GREEN)Updating dependencies...$(NC)"
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

verify: ## Verify dependencies
	@echo "$(GREEN)Verifying dependencies...$(NC)"
	$(GO) mod verify
	@echo "$(GREEN)✓ Dependencies verified$(NC)"

init-db: ## Initialize databases
	@echo "$(GREEN)Initializing databases...$(NC)"
	./$(BINARY_NAME) --init-db || true
	@echo "$(GREEN)✓ Databases initialized$(NC)"

.DEFAULT_GOAL := help
