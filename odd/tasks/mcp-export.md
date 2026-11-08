# MCP ticket export
Issue #33, T8; work branch `feature/mcp-parent-creation`.
Work remains unstaged and uncommitted for maintainer review.

## Specs

- S1: "ok terminemos de implementar entonces el issue 33"
- S2: "ok, recuerda que se tiene que hacer en una rama de trabajo todo nunca sobre develop"
- S3: "era crear todos los commits en una rama y al final hacer dflow finish --delete cada commit era una unidad de la misma rama"
- S4: "todo debe ser tdd"

## Tasks

- [x] T1 (S1-S4; route: worker; commit: this unit): Expose existing CLI board/sprint ticket export via read-only MCP, preserving CLI behavior and truthful completeness, with isolated tests and documentation.
- [x] T2 (S1-S3; route: parent; commit: this unit): Verify and present the unit; user authorized commit without RDD.

## Log

- L1: "ok terminemos de implementar entonces el issue 33"
- L2: "ok, recuerda que se tiene que hacer en una rama de trabajo todo nunca sobre develop"
- L3: "era crear todos los commits en una rama y al final hacer dflow finish --delete cada commit era una unidad de la misma rama"
- L4: "todo debe ser tdd"
- L5: Worker reported separate pre-implementation RED runs for each scenario, but each failed because the entire tool was absent, not because that scenario's rule was missing. Per-rule test-first evidence is therefore not established. Focused tests, go test ./..., go vet ./... and git diff --check passed; the 2,000-issue cap lacks a dedicated test and shared input-parser differential has not been run.
- L6: "esta bien aceptaremos esta unidad asi, solo quiero estar seguro que el proyecto tenga como regla TDD Estricto si esta la regla????" Acceptance is limited to this unit's missing per-rule RED evidence; strict TDD remains required.
- L7: A public isolated test verifies the 2,000-issue cap and another verifies a negative sprintId is rejected with zero Jira requests. The shared parser differential compared all six existing sibling read tools against HEAD using sprintId input; results, stderr and exit codes matched. Full go test ./..., go vet ./... and git diff --check passed; focused negative-sprint test passed afterward.
- L8: User reviewed the unit and instructed: "haz commit sin rdd lo veo bien".
