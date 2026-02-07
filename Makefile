.PHONY: build test clean install fmt lint help

BINARY_NAME=s3spectre
BUILD_DIR=./bin
MAIN_PATH=./cmd/s3spectre

## help: Display this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

## test: Run tests
test:
	@echo "Running tests..."
	@go test -race -v ./...

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean

## install: Install the binary to GOPATH
install:
	@echo "Installing $(BINARY_NAME)..."
	@go install $(MAIN_PATH)

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

## lint: Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run || echo "golangci-lint not installed, skipping..."

## run: Build and run
run: build
	@$(BUILD_DIR)/$(BINARY_NAME)

## tidy: Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy

.DEFAULT_GOAL := help
