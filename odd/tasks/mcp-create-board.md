# MCP create with board
Issue #33 T7; branch `feature/mcp-parent-creation`. Preserve existing creation defaults and #24 scope.
Existing uncommitted TUI user-search changes must be preserved. Work stays unstaged and uncommitted.

## Specs

- S1: "pues terminalo!!! no te detengas entonces si hay mas cosas aun por implementar"
- S2: "solo que en los proximos pasos si hay que aplicar TDD de forma estricta"
- S3: "era crear todos los commits en una rama y al final hacer dflow finish --delete cada commit era una unidad de la misma rama"

## Tasks

- [x] T1 (S1-S3; route: worker; commit: this unit): Add optional board placement to MCP create matching current CLI semantics without changing defaults, with strict per-rule TDD and truthful partial failure.
- [x] T2 (S1-S3; route: parent; commit: this unit): Verify and present this unit; user authorized its separate commit.

## Log

- L1: "pues terminalo!!! no te detengas entonces si hay mas cosas aun por implementar"
- L2: "no, esta bien asi, solo que en los proximos pasos si hay que aplicar TDD de forma estricta"
- L3: "era crear todos los commits en una rama y al final hacer dflow finish --delete cada commit era una unidad de la misma rama"
- L4: Worker reported implementation and focused tests passing, but all initial downstream scenario failures were caused by boardId being rejected as unexpected, and invalid-board/parent-plus-board tests passed before implementation because the old schema rejected boardId universally. Meaningful per-rule RED evidence is missing; T1 remains open. Existing uncommitted TUI user-search changes were preserved.
- L5: User explicitly authorized removing only the new boardId implementation and redoing RED/GREEN: "si". Parent removed the task-created placement block and parent/board conflict guard only, retaining the boardId schema and tests to permit rule-specific RED; TUI user-search changes remain untouched. This correction does not retroactively prove test-first ordering.
- L6: Worker reported corrective RED/GREEN for parent conflict, active placement, no active sprint, lookup failure, rejected placement, unknown placement and unavailable sprint lookup. Invalid ID and fallback placement lacked individual RED evidence. Full go test ./..., go vet ./..., and git diff --check passed. Independent verifier passed functional contracts but could not verify historical strict TDD order. Public MCP differential for boardId on the ten sibling write tools matched responses, stderr and exit codes.
- L7: After disclosure of the limited TDD evidence for this unit, user said: "si lo aceptamos asi". This exception applies only to this unit; strict TDD remains mandatory for future work.
- L8: User authorized separate commits for TUI user search and MCP board creation: "si".
