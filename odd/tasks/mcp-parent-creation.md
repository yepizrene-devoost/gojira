# MCP parent creation
Issue #33, T7; branch `feature/mcp-parent-creation`. Preserve existing creation behavior and issue #24 defaults.
Work remains unstaged and uncommitted for maintainer review.

## Specs

- S1: "ok terminemos de implementar entonces el issue 33"
- S2: "ok, recuerda que se tiene que hacer en una rama de trabajo todo nunca sobre develop"

## Tasks

- [x] T1 (S1-S2; route: worker; commit: pending): Add MCP parent creation matching existing CLI behavior, with isolated tests and documentation.
- [ ] T2 (S1-S2; route: parent; commit: pending): Verify and present this bounded unit for review without staging or committing.

## Log

- L1: "ok terminemos de implementar entonces el issue 33"
- L2: "ok, recuerda que se tiene que hacer en una rama de trabajo todo nunca sobre develop"
- L3: Worker observed RED then GREEN for public MCP parent creation; focused MCP and full Go suites passed, as did git diff --check. Existing MCP schema continues to require an explicit issueType; parent requests require Sub-task. No live Jira calls.
