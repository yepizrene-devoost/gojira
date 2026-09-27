# Issue #17 — Go lint gate

## Objective and scope
Make golangci-lint a CI gate and align local `make lint` with the CI executable version. Issue: https://github.com/yepizrene-devoost/gojira/issues/17. Branch: `feature/ci-golangci-lint` from `develop`.

- Include every Go package via `./...`; no per-package registration. Non-Go files require separate checks.
- Use golangci-lint v2.11.4 and its five default linters (`errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`); no new linter set or autofixes.
- Do not publish branches, merge, close the issue, or configure GitHub branch protection without separate authorization.
- CI only runs on its configured push/PR events; marking the job as a required branch protection check is a separate GitHub setting.

## Tasks and checks
- [x] **L17-1 — Add one cohesive local/CI lint gate.** Delegated writer (four nontrivial files: `.golangci.yml`, `Makefile`, `.github/workflows/ci.yml`, `README.md`). Pin local version and action tool version, add a separate CI lint job using `--timeout=5m`, configure five linters, document invocation. Verify `make lint`, `go test ./...`, and `git diff --check`; check configuration and version mismatch behavior; record environment-limited checks truthfully.
- [ ] **L17-2 — Record work-unit identity.** After verified L17-1 and separate commit authorization, record commit SHA; commit identity bookkeeping may be a short follow-up `docs(odd)` commit. Do not tick before identity exists.

## Route and delivery
One bounded multi-file writer for L17-1 (multi-file write and preparation triggers); verification follows the native assessment plan. Forecast well under 400 authored lines; delivery strategy `ask-on-risk`, one cohesive work unit. TDD mode: not established by project/session settings; use ordinary functional checks, do not invent RED/GREEN evidence. Exact runner: `make lint` (plus `go test ./...`).

## Progress and evidence
- Branch created locally with `dflow start feature ci-golangci-lint --no-push` after confirming clean `develop`.
- Baseline: Makefile runs unversioned golangci-lint; CI build/test/vet only; no `.golangci.*` config. Installed local golangci-lint reports v2.11.4 and its five default linters.
- Implemented a standalone lint CI job (`golangci/golangci-lint-action@v8`, tool v2.11.4, `--timeout=5m`), v2 config with exactly five linters, strict local version check, and README usage/event details.
- Writer: `make lint` passed (0 issues), `go test ./...` passed, `git diff --check` passed, `golangci-lint config verify` passed; mismatch simulation with `make lint GOLANGCI_LINT_VERSION=0.0.0` failed as expected.
- Independent verifier: `make lint`, `go test ./...`, `git diff --check`, `golangci-lint config verify`, `go build ./...` all passed. Verified `version` and `args` inputs in upstream `golangci-lint-action@v8` action metadata. Missing-binary branch was inspected, not executed.
- Hosted CI is pending an authorized publication/PR. GitHub branch protection is not configured and is outside this repository-only work unit.
- User explicitly authorized a local commit and sending this candidate to review. No push, merge, or issue closure authorized. L17-2 stays open until the commit SHA is recorded.
- Review declaration (pre-freeze): commit the complete local lint gate and its identity record before the native review freeze; request native review of the committed range against the branch point. This states the intended review, not its outcome or approval.

## Next step
Create the local work-unit commit, then record its identity before the review freeze; submit only the finalized committed candidate for native review.
