BINARY_NAME := cloudguard
BUILD_DIR := ./bin
CMD_DIR := ./cmd/cloudguard
GO := go
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test lint run clean fmt vet cross

all: lint test build

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

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

cross:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"
