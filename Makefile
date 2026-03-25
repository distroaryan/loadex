.PHONY: all build test clean build-backend build-lb build-cli demo k6 up down install test-e2e test-chaos testsum test-algo lint help

# Binary names
BACKEND_BIN=backend.exe
LB_BIN=loadbalancer.exe
CLI_BIN=loadex.exe

# Build directory
BUILD_DIR=bin

all: clean build test

build: build-backend build-lb build-cli

build-backend:
	@echo "Building backend..."
	@go build -o $(BUILD_DIR)/$(BACKEND_BIN) ./cmd/backend

build-lb:
	@echo "Building loadbalancer..."
	@go build -o $(BUILD_DIR)/$(LB_BIN) ./cmd/loadbalancer

build-cli:
	@echo "Building loadex CLI..."
	@go build -o $(BUILD_DIR)/$(CLI_BIN) ./cmd/loadex

test:
	@echo "Running tests..."
	@go test -count=1 -v -race ./...

test-e2e:
	@echo "Running E2E tests..."
	@go test -count=1 -v -race ./tests/... -run=TestE2E

test-chaos:
	@echo "Running Chaos tests..."
	@go test -count=1 -v -race ./tests/... -run=TestChaos

testsum:
	@echo "Running tests with gotestsum..."
	@gotestsum --format testname -- -count=1 -v -race ./...

test-algo:
	@echo "Running algorithm tests..."
	@go test -count=1 -v -race ./load_balancers/...

lint:
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@go clean

demo:
	@bash scripts/demo.sh

setup:
	@bash scripts/setup.sh

health:
	@bash scripts/health.sh

failover:
	@bash scripts/failover.sh

loadex:
	@./bin/loadex

install: build
	@echo "Installing to GOPATH/bin..."
	@go install ./cmd/loadex
	@go install ./cmd/loadbalancer
	@go install ./cmd/backend

help:
	@echo "Available commands:"
	@echo "  make build          # Build binaries to ./bin/"
	@echo "  make install        # Install to GOPATH/bin"
	@echo "  make clean          # Remove build artifacts"
	@echo "  make lint           # Run golangci-lint"
	@echo "  make test           # Run all tests"
	@echo "  make testsum        # Run all tests using gotestsum"
	@echo "  make test-e2e       # Run integration tests"
	@echo "  make test-chaos     # Run chaos tests"
	@echo "  make test-algo      # Run load balancing algorithm tests"
	@echo "  make help           # Show this help message"
	@echo "  make up             # Start docker cluster"
	@echo "  make down           # Stop docker cluster"
