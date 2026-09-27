# gojira Makefile

BINARY_NAME   = gojira
BIN_DIR       = bin
CHANGELOG    ?= CHANGELOG.md
RELEASE_NOTES ?= RELEASE_NOTES.md
GOLANGCI_LINT_VERSION = 2.11.4

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

.PHONY: build-all
build-all:
	@mkdir -p "$(BIN_DIR)"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o "$(BIN_DIR)/$(BINARY_NAME)-linux-amd64" .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o "$(BIN_DIR)/$(BINARY_NAME)-linux-arm64" .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o "$(BIN_DIR)/$(BINARY_NAME)-darwin-amd64" .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o "$(BIN_DIR)/$(BINARY_NAME)-darwin-arm64" .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o "$(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe" .
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o "$(BIN_DIR)/$(BINARY_NAME)-windows-arm64.exe" .

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint v$(GOLANGCI_LINT_VERSION) is required but is not installed" >&2; exit 1; }
	@installed_version=$$(golangci-lint version 2>/dev/null | awk 'NR == 1 { print $$4 }'); \
	if [ "$$installed_version" != "$(GOLANGCI_LINT_VERSION)" ]; then \
		echo "golangci-lint version mismatch: required v$(GOLANGCI_LINT_VERSION), found v$${installed_version:-unknown}" >&2; \
		exit 1; \
	fi
	golangci-lint run ./...

.PHONY: changelog
changelog:
	@command -v git-cliff >/dev/null 2>&1 || { echo "git-cliff not installed"; exit 1; }
	git-cliff --config cliff.toml --output CHANGELOG.draft.md

.PHONY: release-notes
release-notes:
	@tmp="$(RELEASE_NOTES).tmp"; \
	rm -f "$$tmp" "$(RELEASE_NOTES)"; \
	if awk -v tag="$(VERSION)" ' \
		BEGIN { found = 0; content = 0 } \
		$$1 == "##" && $$2 == "📦" { \
			if (found) exit; \
			if ($$3 == tag && tag ~ /^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$$/) { found = 1; next } \
			exit 1 \
		} \
		found { print; if ($$0 ~ /[^[:space:]]/) content = 1 } \
		END { if (!found || !content) exit 1 } \
	' "$(CHANGELOG)" >"$$tmp" && mv "$$tmp" "$(RELEASE_NOTES)"; then \
		:; \
	else \
		rm -f "$$tmp" "$(RELEASE_NOTES)"; \
		echo "release notes rejected: newest package section must match $(VERSION) and contain content" >&2; \
		exit 1; \
	fi

.PHONY: release
release:
	@command -v goreleaser >/dev/null 2>&1 || { echo "goreleaser not installed" >&2; exit 1; }; \
	if [ ! -r .env ]; then echo ".env is missing or unreadable" >&2; exit 1; fi; \
	set -a; . ./.env; set +a; \
	if [ -z "$${GITHUB_TOKEN:-}" ]; then echo "GITHUB_TOKEN is missing from .env" >&2; exit 1; fi; \
	if [ "$(VERSION)" = dev ]; then echo "release version cannot be dev" >&2; exit 1; fi; \
	tag_commit=$$(git rev-parse --verify "refs/tags/$(VERSION)^{commit}" 2>/dev/null) || { echo "release version $(VERSION) must be an exact local tag on HEAD" >&2; exit 1; }; \
	head_commit=$$(git rev-parse --verify HEAD 2>/dev/null) || { echo "release version $(VERSION) must be an exact local tag on HEAD" >&2; exit 1; }; \
	if [ "$$tag_commit" != "$$head_commit" ]; then echo "release version $(VERSION) must be an exact local tag on HEAD" >&2; exit 1; fi; \
	$(MAKE) --no-print-directory release-notes VERSION="$(VERSION)" CHANGELOG="$(CHANGELOG)" RELEASE_NOTES="$(RELEASE_NOTES)" || exit 1; \
	goreleaser release --clean --release-notes="$(RELEASE_NOTES)"

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
