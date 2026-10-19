# Multiplatform installer scripts

## Objective and scope
Add release-asset installer scripts for GoJira on Linux, macOS, and Windows, following the local dflow pattern, without publishing a release. The broad README accuracy pass is a separate later work unit requested by the user.

- Publish standalone `scripts/install.sh` and `scripts/install.ps1` with the next separately authorized `make release`; keep the local-only publishing policy.
- Pin explicit GoReleaser archive names for amd64/arm64 on all three OSes; resolve the latest or a selected version, verify SHA-256 before extraction, reject unsafe archives/destinations and install without sudo.
- Default to `$HOME/.local/bin` on POSIX and `%LOCALAPPDATA%/Programs/gojira` on Windows; support version and install-directory overrides, with safe PATH handling.
- Run hermetic POSIX and Windows fixture checks in CI. No real release download/install in fixtures. No README overhaul, package managers, signing, self-update, PR, merge or release publication in this unit.

## Checklist
- [x] I1: Establish stable GoReleaser asset names and include installers as release assets; `goreleaser check` and contract assertions passed.
- [x] I2: Implement checksum-verified POSIX and Windows installers with safe replacement and PATH behavior; isolated fixtures cover success, checksum ambiguity/mismatch, unsafe archive entries and unsafe destinations; Windows PowerShell 5.1 and 7 fixtures passed in CI.
- [x] I3: Wire portable checks into CI; Ubuntu release/POSIX checks and both Windows shells passed in run 36355300430 for commit `217347101c225dd2aae5d14d2ac2663bae9b0425`. Local `go test ./...` and `go vet ./...` passed.
- [x] I4: Record work-unit commits and pre-freeze declaration. Original work-unit commit: `fee66b19276741dbbf3cc2135951db02e086a976`; Windows fixture correction: `217347101c225dd2aae5d14d2ac2663bae9b0425`. Review is **not** approved or granted; the review candidate will be the committed range from `develop` after this bookkeeping lands, and its outcome will be stored in the native receipt/Engram, not written into the reviewed tree.

## Work plan and evidence
Branch: `feature/multiplatform-installers` from `develop` via dflow. Route: delegated direct writer and independent verifier for multi-file work. TDD mode not configured; ordinary focused tests. The user selected one cohesive installer work unit despite 877 added lines and separately authorized each local CI-candidate commit, branch push and workflow dispatch. No PR, merge or release authorized.

Read-only mapping found six static build targets and implicit asset names. Independent verification initially found unsafe tar links, directory destination false success, PowerShell 5.1 fixture binding and duplicate-checksum issues; corrected with negative fixtures. Local `sh tests/installer-contract.sh`, `goreleaser check`, `go test ./...`, `go vet ./...` and `git diff --check` passed; POSIX tar emits a nonfatal extended-attributes warning on this macOS host. `pwsh` is unavailable locally. The first manual CI run 36355147352 passed Ubuntu but failed Windows PowerShell 5.1 due to fixture scope; correction in `2173471` passed the second manual run 36355300430 on Ubuntu and Windows PowerShell 5.1/7. These fixture runs do not constitute a real published-release installation or authenticate checksums independently of GitHub release provenance.

Pre-freeze bookkeeping closes the implementation stages without claiming native approval. After a separately authorized local bookkeeping commit, assess and review the exact committed range against base `1ee74895571790d9f42b8ee469fa36a825a81660`; the prior assessment of the initial commit was high risk. After approval there are zero further commits or tree writes. No release publication is authorized. Rollback boundary: revert this work unit's installer scripts, their fixtures, GoReleaser asset naming/publication entries, and CI checks together; do not remove unrelated release machinery.
