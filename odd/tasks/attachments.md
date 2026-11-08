# Attachments for CLI, MCP and TUI
Branch `feature/attachments` from develop a8dc102. Add a file to a Jira issue from the three surfaces.
Strict TDD: a specific RED before each rule, then GREEN. Work stays uncommitted until the maintainer asks for each commit.

## Specs

- S1: "ok lo que quiero ahora es la capacidad de agregar attachments tanto para mcp, CLI y tui"
- S2: "y recuerdalo con TDD strict"
- S3: "2 cualquier ruta legible sin tema"

## Tasks

- [x] T1 (S1,S2; route: parent; commit: b1cedeb): Jira client: attachment settings and one multipart upload per file, in `internal/jira`, endpoints in `paths.go`.
- [x] T2 (S1,S2; route: parent; commit: b1cedeb): shared logic in `internal/attachments`: path validation, settings preflight, one request per file, per-file mutation state.
- [x] T3 (S1,S2; route: parent; commit: b1cedeb): CLI `gojira attach <issue-key> <file>...` with `--json`.
- [x] T4 (S1-S3; route: parent; commit: pending): MCP write tool `gojira_attach` behind `--allow-writes`, accepting any readable path.
- [x] T5 (S1,S2; route: parent; commit: pending): TUI path prompt on key `f` in the ticket detail.
- [x] T6 (S1; route: parent; commit: pending): README, help text and the parity matrix.

## Log

- L1: "ok lo que quiero ahora es la capacidad de agregar attachments tanto para mcp, CLI y tui"
- L2: Analysis, from the Jira Cloud docs: `POST /rest/api/3/issue/{key}/attachments` takes multipart/form-data with the part named `file` and the header `X-Atlassian-Token: no-check`; `GET /rest/api/3/attachment/meta` returns `enabled` and `uploadLimit` in bytes. The existing `mutate` sends JSON once and never retries; uploads need a multipart variant with the same single-attempt and three-state rules.
- L3: "y recuerdalo con TDD strict"
- L4: Security decision asked for the MCP, where a path argument lets an agent upload any file the process can read. The maintainer answered: "2 cualquier ruta legible sin tema". Accepted risk: the MCP tool reads any readable path, with no allowlist. It stays behind `--allow-writes`, its description states the risk, and it still refuses anything that is not a regular file.
- L5: Design: one request per file because Jira does not document multi-file atomicity; all paths are validated and the size limit checked before the first upload; if a metadata read fails nothing is uploaded; uploads are never retried; the MCP takes one `path` per call, the CLI takes several files.
- L6 (work unit A, core and CLI, T1-T3 only): Evidence. T1: RED/GREEN for settings and multipart upload; the outcome-state and single-attempt test passed on first run (inherited from `dispatchMutation`), so it has no RED. T2: 8 tests, RED observed against an empty stub. T3 (worker): RED observed for rules 1-5. T4 (worker): real RED for rules 1-2; rules 3-5 have no prior RED, rules 4-5 only checked with a later temporary mutation. T5 (worker): real RED for rules 1-4 and 6; rule 5 has no RED because the error handling already existed from rule 2. The maintainer accepted these TDD exceptions.
- L7 (work unit B): Known gaps: the MCP `Guide` text still says there are no write tools (pre-existing, now less accurate); no real Jira run and no manual TUI session. Independent verification found that Esc discarded an in-flight upload result and unknown outcomes were not identified in the TUI; both were corrected and independently verified. The verifier also found trimmed paths; the TUI now preserves input exactly, with an isolated test for a trailing-space filename.
- L10: TUI corrections: the worker reported behavioral RED/GREEN for in-flight Escape and unknown-outcome feedback, although the latter test assertion changed after its RED; a separate RED/GREEN covered preserving path whitespace. The final independent verifier passed isolated MCP/TUI tests and rendered warning checks; it did not independently observe historical RED runs or test a path starting with whitespace.
- L9: The maintainer accepted the documented TDD exceptions and the informational findings of the second review of unit A, then explicitly authorized commit b1cedeb. Unit B restores MCP and TUI changes and their documentation; build, vet, full tests, lint and diff checks passed before review. Native review of unit B is requested.
- L8 (work unit A, after the first native review): the review approved with three advisory warnings and the maintainer chose to fix them before committing. Four tests were written first and observed failing, then fixed: a partial failure after a rejected upload reports `applied` instead of `not_applied` (create already uses `applied` for partial_failure); uploaded files are named from the local path, not from Jira's echo; a success with no Jira details still prints one line per file; malformed JSON in a fully read 2xx response is not turned into a failure because Jira already accepted the file. A response-body read error remains an applied mutation error.
