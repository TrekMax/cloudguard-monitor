BINARY_NAME := cloudguard-cli
BUILD_DIR := ./bin
CMD_DIR := ./cmd/cloudguard-cli
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
