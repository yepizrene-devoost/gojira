# Issue #12 — Surface Jira response body read errors

- [x] Add regression coverage proving response-body read failures are returned.
- [x] Run focused and broader Go verification.
- [ ] Record work-unit commit identity and review evidence.

## Evidence

- Existing implementation: `internal/jira/client.go:64-67` returns the `io.ReadAll` error from the shared request path.
- Verification: `go test ./internal/jira` — passed.
- Verification: `go test ./...` — passed.
- Verification: `go vet ./...` — passed.
- Formatting: `gofmt -l internal/jira/client.go internal/jira/client_test.go` — clean.
- Diff check: `git diff --check` — passed.
- Runtime harness: N/A; this unit changes Jira client tests and has no separate local runtime harness.
- Rollback boundary: revert the issue-12 work-unit commit to remove the regression test and task evidence.

## Scope

- `internal/jira/client_test.go`
- `odd/tasks/issue-12-read-errors.md`
- `odd/issue-12-read-errors/tasks`
