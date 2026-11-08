# Issue #33 closeout: T4b and T9
Issue #33; branch `feature/issue-33-closeout` from develop 74ae393. Closes the two ledger tasks left open after PR #36.
Work stays unstaged and uncommitted for maintainer review; closing the issue is a separate confirmation.

## Specs

- S1: "hay que implementar t4b y t9 necesitamos cerrar por completo el issue #33"
- S2: T4b, as recorded in `odd/tasks/interface-parity.md`: independent verification of T2 (MCP update-field parity, risk item 3) and T3 (TUI assignment, risk item 4). One new read-only verifier run; if it fails it is reported as unavailable and not retried.
- S3: T9, as recorded in `odd/tasks/interface-parity.md`: final matrix and verification across all delivered units; release or delivery only under separate authorization.
- S4: AGENTS.md: closing an issue is a standalone decision with its own explicit confirmation that names the exact action; a merged PR or "cerrar por completo" does not authorize it.

## Tasks

- [x] T1 (S2; route: independent verifier; commit: pending): one independent verification of v0.3.0 parity unit 1 against S3, S4 and S6 of the parity ledger.
- [x] T2 (S3; route: parent; commit: pending): cross-check every row of `docs/interface-parity.md` against the code, run the full checks on this tree, and list the evidence gaps that remain.
- [ ] T3 (S1-S3; route: parent; commit: pending): reconcile `odd/tasks/interface-parity.md` (T4b, T9) and the issue #33 checklist, ticking only what has evidence. The ledger part is done (L6); the issue checklist is still pending and needs the maintainer's go-ahead.
- [ ] T4 (S4; route: parent; commit: pending): ask for commit, PR and the issue closure as separate confirmations.

## Log

- L1: "hay que implementar t4b y t9 necesitamos cerrar por completo el issue #33"
- L2: T4b, one independent verifier run (read-only, task mw7b5zce) of the v0.3.0 first unit. T2 (MCP `gojira_update` fields through `internal/issuefields`, shared with `cmd/update.go`): S3 pass, with preparation, clear semantics, validation and metadata checks before mutation. T3 (TUI assignment `A`, `internal/tui/assign.go`): S4 pass, with one identity lookup and one mutation, no retry, cancellation never submits and stale replies are ignored. S6: the final-tree tests pass, historical RED cannot be established from the tree. `go test ./internal/issuefields ./internal/mcpserver ./internal/tui ./cmd -count=1` passed. No live Jira or manual TUI session. The verifier made no edits.
- L3: T9 matrix cross-check against the code. All 20 MCP tools named in the matrix exist; the code defines 11 write tools and every one is in the matrix. Three apparent extras (`gojira_description`, `gojira_key`, `gojira_project`) were false positives of my first regular expression, which read property names as tools; counting only inside `writeTools()` found none. All 20 CLI commands of the matrix answer `--help` with exit 0. Every TUI key of the matrix has a `case` in `tui.go`, and `Ctrl+L` is present. Key presence is not proof of behavior; behavior is covered by the per-unit tests.
- L4: `docs/interface-parity.md` has no row left as Partial, No or In progress. Its introduction said the matrix was not a claim of full parity; it now says every row is implemented on the three surfaces, that row limits are written in the row, and that project listing in the TUI is the one exclusion on purpose. The status vocabulary table stays for future gaps.
- L5: Full checks on this tree (job-22): `go build`, `go vet`, `go test ./... -count=1`, `make lint` (0 issues, unpiped) and `git diff --check` pass. Caveats that remain, all disclosed in the per-unit ledgers and accepted by the maintainer unit by unit: historical TDD gaps; no native RDD on PR #36 (maintainer decision, Boreas reviewed instead); no independent verifier for the TUI export (quota); `TestCommentEntry` and `TestLifecycleDeleteFailures` fail with `-race` on base f8f37e4 and CI does not run `-race`; no live Jira or manual TUI session in any unit.
- L6: `odd/tasks/interface-parity.md` reconciled: T4b and T9 ticked with pointers to L2-L5, and L17-L18 added. The issue #33 checklist and its closure are not touched.
- L7: Maintainer instruction after T4b and T9 were presented, verbatim: "haras commit, dflow finish --delete y luego cerraras el issue como completo". The commit and `dflow finish --delete` are authorized by it. Closing issue #33 is kept as a separate confirmation naming the exact action, as AGENTS.md requires, and is not done by this commit.
