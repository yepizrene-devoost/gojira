# gojira Makefile

BINARY_NAME = gojira
BIN_DIR     = bin

# Channel marker only. A tag is injected when HEAD is exactly on it; every
# other revision keeps the `dev` marker, so a development tree never claims a
# released version. The installed commit is deliberately NOT injected here:
# the binary self-reports it from its own VCS stamp (buildvcs stays on).
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo dev)

LDFLAGS = -X main.version=$(VERSION)

.PHONY: all
all: build

.PHONY: build
build:
	@echo "Building $(BINARY_NAME) (version: $(VERSION))..."
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) .

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed"; exit 1; }
	golangci-lint run ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: check
check: vet test

.PHONY: install
install:
	go install -ldflags="$(LDFLAGS)" .

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)
	rm -f $(BINARY_NAME)
