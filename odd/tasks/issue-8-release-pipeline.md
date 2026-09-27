# Issue #8 — Release pipeline

## Objective
Publish reproducible cross-platform release archives with a visible version, automated validation, and an operator-ready release guide.

## Context and scope
- Issue: https://github.com/yepizrene-devoost/gojira/issues/8
- Branch: `feature/issue-8-release-pipeline` from `develop`.
- Existing: `main.version`, `gojira version [--json]`, Makefile ldflags, and `cliff.toml`.
- In scope: version regression coverage, PR/push CI, GoReleaser v2 builds and archives, tag release automation, `CHANGELOG.md`, and `RELEASING.md`.
- Out of scope: optional installer scripts, optional lint service, package managers, signing, and publishing any release in this task.
- Use `vX.Y.Z` tags on the merged `main` commit. Preserve the public `v` prefix with `main.version={{ .Tag }}`. Never embed credentials; CI receives the ephemeral GitHub token.
- TDD: not enabled by observed repository/session configuration; use focused tests and build checks. Exact runner: `go test ./...`; configuration validation: `goreleaser check` if the binary is available. No implicit TDD from existing tests.
- Delivery strategy: `ask-on-risk`; estimated 250–400 authored diff lines; assess running count before each work-unit commit. No push, merge, or release without separate user confirmation.

## Tasks
- [x] R8-1 — Lock the existing version contract in tests and add build/test/vet CI for pushes to `develop`/`main` and pull requests. Route: delegated writer (multiple non-trivial files). Checks: `go test ./cmd -run 'TestVersion' -count=1`, `go test ./...`, `go vet ./...`, `git diff --check`, `go run . version --json` all passed; inspected workflow triggers and Go version file. GitHub-hosted execution pending first run. Commit identity: pending R8-4 follow-up.
- [x] R8-2 — Add GoReleaser v2 cross-platform builds, archives, checksums, and tag-only release workflow. Route: delegated writer (multiple non-trivial files). Checks: `goreleaser check` passed (v2.18.2); `goreleaser release --snapshot --clean` produced six archives, all with binary/README/LICENSE/CHANGELOG and valid SHA-256 checksums; snapshot binary reported `v0.0.1-next`; `go test ./...`, `go vet ./...`, `go build ./...` passed. Independent verifier rechecked config, version, tag/main guard, test/vet, and found no blocker. GitHub-hosted release execution pending first authorized tag. Commit identity: pending R8-4 follow-up.
- [x] R8-3 — Publish initial changelog placeholder and concise release guide aligned with dflow and README; document tag, manual promotion, dry run, and verification. Route: delegated writer (multiple non-trivial files). Checks: writer and independent verifier inspected doc/config alignment; `git diff --check` passed. Commit identity: pending R8-4 follow-up.
- [ ] R8-4 — Record work-unit commit identities, final checks, and freeze expectation in this file before final review. Commit identity is the permitted follow-up `docs(odd)` commit if required; no post-approval writes. Route: inline bookkeeping. Checks: commit range, clean worktree, final review candidate and native outcome. Evidence: pending.

## Progress and evidence
- `ed020dc4a064ee8c3b923dd683f95de3b9e43426` — previously reviewed `AGENTS.md` session/branch guardrails, outside issue #8 release implementation.
- R8-1: `c450238f23d2f456d0063c21247aa187242c233c` added `cmd/version_test.go` and `.github/workflows/ci.yml`. Native review `review-47610e0eb4240b7c` approved and acknowledged; advisory test observations were non-blocking. Rollback: remove these two new files without changing existing version behavior.
- R8-2/R8-3: added `.goreleaser.yaml`, `.github/workflows/release.yml`, `CHANGELOG.md`, `RELEASING.md`, and updated `README.md`. Writer's snapshot produced and verified six ignored `dist/` archives; independent verifier's focused checks passed. Runtime harness: snapshot `gojira version --json` returned `v0.0.1-next`. The release workflow's `vX.Y.Z` guard and main ancestry check are in place. `actionlint` unavailable and live Actions untested. Rollback: revert/remove only these five files; `dist/` is generated and ignored.

## Next step
Commit the closed R8-2/R8-3 combined work unit. Record its commit identity and the freeze expectation in R8-4 before the final native review; no post-approval writes.
