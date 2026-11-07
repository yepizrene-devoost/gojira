# MCP assignment by email
Issue #33, T8; branch `feature/mcp-assign-email`. Preserve direct account ID assignment.
Work remains unstaged and uncommitted for maintainer review.

## Specs

- S1: "ok terminemos de implementar entonces el issue 33"
- S2: "ok, recuerda que se tiene que hacer en una rama de trabajo todo nunca sobre develop"
- S3: "ok haz commit sin rdd y continua con la siguiente tarea"

## Tasks

- [x] T1 (S1-S3; route: worker; commit: this unit): Add email-based MCP assignment with existing Jira identity resolution and retain accountId behavior; include isolated tests and docs. Missing per-rule RED evidence disclosed and accepted for this commit.
- [x] T2 (S1-S3; route: parent; commit: this unit): Verify and report the unit for review; user authorized commit without RDD.

## Log

- L1: "ok terminemos de implementar entonces el issue 33"
- L2: "ok, recuerda que se tiene que hacer en una rama de trabajo todo nunca sobre develop"
- L3: "ok haz commit sin rdd y continua con la siguiente tarea"
- L4: Email assignment and schema had observed RED/GREEN. Worker reported that separate RED was not captured for each already-implemented validation and failure rule. Full go test ./..., go vet ./..., focused MCP tests and git diff --check passed.
- L5: After disclosure of missing per-rule RED evidence, user instructed: "ok haz commit sin rdd".
