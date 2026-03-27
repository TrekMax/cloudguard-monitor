BINARY_NAME := cloudguard
BUILD_DIR := ./bin
CMD_DIR := ./cmd/cloudguard
GO := go

.PHONY: all build test lint run clean fmt vet

all: lint test build

build:
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

test:
	$(GO) test ./... -v -race -count=1

lint: fmt vet
	@echo "lint passed"

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

clean:
	rm -rf $(BUILD_DIR)

coverage:
	$(GO) test ./... -coverprofile=coverage.out -race
	$(GO) tool cover -html=coverage.out -o coverage.html
