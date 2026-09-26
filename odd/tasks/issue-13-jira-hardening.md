# Issue 13 — Jira HTTP hardening

- [ ] Implement bounded retries with backoff for transient network errors, HTTP 429, and 5xx responses.
- [ ] Honor `Retry-After` when present and keep non-transient failures immediate.
- [ ] Escape issue-key path segments across Jira REST endpoints.
- [ ] Add focused regression tests for retry behavior and path escaping.
- [x] Run focused and full Go verification.
- [x] Analyze TUI board-selection latency and identify optimization options.
- [ ] Record work-unit commit identity and review evidence.

## Evidence

- Focused tests: `go test ./internal/jira` — passed.
- Full tests: `go test ./...` — passed.
- Formatting: `gofmt -l internal/jira/client.go internal/jira/client_test.go` — clean.
- Runtime harness: N/A; this unit changes the Jira client and has no separate local runtime harness.
- Latency analysis: TUI board selection performs sequential config, sprint, and issue requests; optimization deferred as a separate measured change.
- Rollback boundary: revert the issue-13 work-unit commit to remove retry/escaping behavior and its tests.
