# Issue #15 — JQL encoding and comment date safety

- [x] Fix `SearchJQL` to URL-encode the complete JQL query and add regression coverage.
- [x] Guard comment date rendering against missing or short timestamps and add regression coverage.
- [x] Run focused and broader Go checks; record evidence and commit identity.

## Evidence

- `git diff --check` — passed.
- `go test ./cmd` — passed.
- `go test ./internal/jira` — passed.
- `go test ./...` — passed.
- Commit identity: pending explicit commit authorization.

## Scope

- `internal/jira/client.go`
- `internal/jira/client_test.go`
- `cmd/get.go`
- `cmd/get_test.go`

## Acceptance criteria

- JQL containing spaces, `&`, `%`, `+`, `#`, and quotes reaches the server as the original query.
- Rendering a comment with an empty or short `Created` value never panics and emits a safe value.
- Existing behavior remains unchanged for normal Jira timestamps.
