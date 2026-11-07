# TUI JQL search
Issue #33 T8; branch `feature/mcp-parent-creation`. Preserve text search behavior.
Work stays unstaged and uncommitted for maintainer review.

## Specs

- S1: "hay que implementar todo entonces"
- S2: "todo debe ser tdd"
- S3: "era crear todos los commits en una rama y al final hacer dflow finish --delete cada commit era una unidad de la misma rama"

## Tasks

- [x] T1 (S1-S3; route: worker; commit: this unit): Add TUI JQL search using existing Jira search and truthful truncation, preserving existing text search; per-rule strict TDD and documentation.
- [x] T2 (S1-S3; route: parent; commit: this unit): Verify and present the unit; user authorized commit.

## Log

- L1: "hay que implementar todo entonces"
- L2: "todo debe ser tdd"
- L3: "era crear todos los commits en una rama y al final hacer dflow finish --delete cada commit era una unidad de la misma rama"
- L4: Worker reported individual RED/GREEN for mode display/switch, unchanged JQL routing, stale response invalidation, default mode, board filter routing, cancel restoration, help and mode-aware placeholders. Existing text, errors and truncation passed as regression checks. Parent ran go test ./..., go vet ./... and git diff --check successfully.
- L5: Independent verifier passed functional scope with coverage caveat: stale success/error tests dispatch messages sequentially, not under real timing interleavings. Verifier passed go test ./internal/tui and ./internal/jira; could not independently infer RED order from final tree. Per-rule execution evidence is the worker's reported commands, not inferred from test presence. No live Jira or manual TUI.
- L6: User reviewed the unit and instructed: "haz el commit".
