.PHONY: all build test clean run-backend run-lb run-cli

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
	@go test -v ./...

clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@go clean

run-backend:
	@go run ./cmd/backend

run-lb:
	@go run ./cmd/loadbalancer

run-cli:
	@go run ./cmd/loadex
