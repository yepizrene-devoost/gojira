# Issue #14 — Config and token storage safety

- [x] Add focused config and token storage tests with deterministic keyring seams.
- [x] Propagate token/config persistence errors and handle wizard input read errors.
- [x] Run focused and broader Go checks; record evidence and commit identity.

## Evidence

- `go test ./internal/config` — passed.
- `go test ./cmd` — passed.
- `go test ./...` — passed.
- `go vet ./...` — passed.
- `git diff --check` — passed.
- Native RDD reliability review — approved; authority burned.
- Work-unit commit: `3fe360a` (`fix(config): cover token storage and input errors`).

## Scope

- `internal/config/config.go`
- `internal/config/config_test.go`
- `cmd/config.go`
- `cmd/config_test.go` (if needed for wizard input coverage)

## Acceptance criteria

- Config save/load round trips are covered, including missing and malformed files.
- Keyring success, keyring failure fallback, and persistence failures are covered without real keychain access.
- Interactive config initialization never ignores `ReadString` errors.
- `SaveToken` reports config persistence failures after keyring writes.
- Focused and full Go checks pass.
