# TUI board and sprint export
Issue #33 T8; branch `feature/mcp-parent-creation`. Preserve existing column and detail copying.
Work stays unstaged and uncommitted for maintainer review.

## Specs

- S1: "la exportación si hay que hacer la par en CLI y MCP"
- S2: "con dejarlo en el portapapeles es suficiente pienso"
- S3: "solo que en los proximos pasos si hay que aplicar TDD de forma estricta"
- S4: "el listado de los proyectos en la TUI no lo necesitamos asi que descartamos eso de la paridad"

## Tasks

- [x] T1 (S1-S4; route: worker; commit: this unit): Add explicit TUI board/sprint JSON export to clipboard with visible scope, honest completeness and strict per-rule TDD; record TUI project listing as intentionally excluded.
- [x] T2 (S1-S4; route: parent; commit: this unit): Verify inline and commit; the independent verifier was unavailable (quota) and the user asked not to delegate.

## Log

- L1: "el listado de los proyectos en la TUI no lo necesitamos asi que descartamos eso de la paridad"
- L2: "la exportación si hay que hacer la par en CLI y MCP"
- L3: "con dejarlo en el portapapeles es suficiente pienso"
- L4: "no, esta bien asi, solo que en los proximos pasos si hay que aplicar TDD de forma estricta"
- L5: User authorized `/Users/rene/Library/Caches/go-build` and `/var/folders/70/5q3z_90x3n39bvpvc_yty3bh0000gn/T/gpbayZfK` for this export unit's tests: "si". Runtime and sandbox guards remain binding.
- L6: Worker reported specific RED then GREEN for the view and scope, board and sprint entry and validation, default active-sprint resolution with all-board fallback, explicit overrides, curated JSON copy, completeness warning, fetch and discovery errors, stale-scope invalidation and queued-write cancellation. Tests that passed on their first run, reported as regression coverage and not as RED: clipboard failure visibility, existing `C`/`c`/`E` shortcuts, board-navigation stale response, already-started clipboard write ignored, old board sprint resolution discarded, no-active-sprint fallback.
- L7: User asked to continue without delegating: "continua tu el trabajo sin delegar". The independent verifier for this high-risk unit (risk item 4) failed with a quota error and was not retried. Inline review of the async handlers found no defect; `exportContext` is only set before the single command that produces a fetch message.
- L8: Parent verification: go build, go vet, go test ./... -count=1, make lint (0 issues, exit 0 with no pipe) and git diff --check pass. Race detector passes on internal/tui and internal/mcpserver except TestCommentEntry and TestLifecycleDeleteFailures, which fail identically on base f8f37e4 and are not run by CI. Formatting was applied only to lines added by this unit because tui.go was already gofmt-dirty on the base; a whole-file gofmt would churn about 1,000 unrelated lines. Alignment-only changes to neighboring struct fields are whitespace-only (git diff -w shows no deletions).
- L9: User decisions: clipboard only, no file export; TUI project listing intentionally excluded from parity; PR to develop for Boreas review instead of dflow finish --delete. Commit authorized for the completed tree: "haz commit de lo que esta en el arbol si ya esta completa la implementación".
