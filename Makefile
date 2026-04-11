# tzc Makefile — formatter, linter, tests, build (outputs under build/)

BUILD_DIR := build
GOLANGCI_LINT_VERSION := v2.11.3
GOLANGCI_LINT := $(BUILD_DIR)/golangci-lint

# Keep all Go build/test cache and temp files under BUILD_DIR so `make clean` is sufficient.
export GOCACHE := $(CURDIR)/$(BUILD_DIR)/go-cache
export GOTMPDIR := $(CURDIR)/$(BUILD_DIR)/go-tmp

.PHONY: fmt lint test build clean all

# Default: format, lint, test, then produce the binary
all: fmt lint test build

# Check that all Go files are formatted (fails if any need formatting)
fmt:
	@test -z "$$(gofmt -l .)" || (echo "These files need formatting (run: gofmt -w .):"; gofmt -l .; exit 1)

# Download golangci-lint binary if not present at the pinned version
$(GOLANGCI_LINT):
	@mkdir -p $(BUILD_DIR)
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(BUILD_DIR) $(GOLANGCI_LINT_VERSION)

# Run golangci-lint (cache also lives under build/)
lint: $(GOLANGCI_LINT)
	@mkdir -p $(BUILD_DIR)/go-tmp $(BUILD_DIR)/golangci-lint-cache
	@GOLANGCI_LINT_CACHE=$(CURDIR)/$(BUILD_DIR)/golangci-lint-cache $(GOLANGCI_LINT) run

test:
	@mkdir -p $(BUILD_DIR)/go-tmp
	@go test ./...

build:
	@mkdir -p $(BUILD_DIR)/go-tmp
	@go build -o $(BUILD_DIR)/tzc .

clean:
	@rm -rf $(BUILD_DIR)
