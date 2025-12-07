.PHONY: help format lint build test clean install-tools

# Variables
GO := go
GOFMT := gofmt
GOLINT := golangci-lint
BINARY_NAME := shopify-demo
BUILD_DIR := bin
MAIN_PACKAGE := ./app
GOBIN := $(shell go env GOBIN)
GOPATH := $(shell go env GOPATH)
ifeq ($(GOBIN),)
GOBIN := $(GOPATH)/bin
endif
export PATH := $(GOBIN):$(PATH)

# Default target
help:
	@echo "Available targets:"
	@echo "  make format      - Format Go code using gofmt"
	@echo "  make lint        - Run linter (golangci-lint)"
	@echo "  make build       - Build the application (order_graphql.go)"
	@echo "  make build-lib   - Build library (package app)"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make install-tools - Install required tools (golangci-lint)"
	@echo "  make all         - Run format, lint, and build"

# Format Go code
format:
	@echo "Formatting Go code..."
	@$(GOFMT) -w $(MAIN_PACKAGE)/*.go
	@echo "✓ Formatting complete"

# Lint Go code - lint all .go files automatically
lint:
	@echo "Running linter on all Go files..."
	@if command -v $(GOLINT) > /dev/null 2>&1 || [ -f $(GOBIN)/$(GOLINT) ]; then \
		GOLINT_CMD=$$([ -f $(GOBIN)/$(GOLINT) ] && echo "$(GOBIN)/$(GOLINT)" || echo "$(GOLINT)"); \
		for gofile in $(MAIN_PACKAGE)/*.go; do \
			if [ -f "$$gofile" ]; then \
				echo "Linting $$(basename $$gofile)..."; \
				$$GOLINT_CMD run "$$gofile" || exit 1; \
			fi; \
		done \
	else \
		echo "⚠ golangci-lint not found. Installing..."; \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		for gofile in $(MAIN_PACKAGE)/*.go; do \
			if [ -f "$$gofile" ]; then \
				echo "Linting $$(basename $$gofile)..."; \
				$(GOBIN)/$(GOLINT) run "$$gofile" || exit 1; \
			fi; \
		done \
	fi
	@echo "✓ Linting complete"

# Build the application (order_graphql.go - package main)
build:
	@echo "Building application (order_graphql.go)..."
	@mkdir -p $(BUILD_DIR)
	@$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)/order_graphql.go
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build library (package app)
build-lib:
	@echo "Building library (package app)..."
	@mkdir -p $(BUILD_DIR)
	@$(GO) build -o $(BUILD_DIR)/app.a -buildmode=archive $(MAIN_PACKAGE)/createOrder.go
	@echo "✓ Library build complete: $(BUILD_DIR)/app.a"

# Run tests
test:
	@echo "Running tests..."
	@$(GO) test -v ./...
	@echo "✓ Tests complete"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@$(GO) clean
	@echo "✓ Clean complete". 

# Install required tools
install-tools:
	@echo "Installing required tools..."
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(GO) install golang.org/x/tools/gopls@latest
	@echo "✓ Tools installed"

# Run all: format, lint, and build
all: format lint build
	@echo "✓ All tasks complete"

# Check code before commit (format + lint)
check: format lint
	@echo "✓ Code check complete"

