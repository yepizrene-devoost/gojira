# Changelog

GoJira release notes are curated from changes merged into the project. The
heading of the newest package section is also the source for local release
publishing.

## 📦 v0.1.1 – Documentation update

This release changes documentation only. The binaries contain no code changes
from v0.1.0.

### Documentation

- Reordered the README so installation, API token, and configuration come
  before the CLI reference, TUI shortcuts, and the agent-oriented sections
  (project context, MCP server, reports, and the optional agent guide).
- Updated the README installation section now that v0.1.0 is published, with
  pinned-version examples.
- Documented that release tags in this repository are signed, annotated tags,
  and how to create and verify one without opening an editor.

## 📦 v0.1.0 – First public release

### Interactive Jira workflow

- Added a keyboard-first Kanban TUI with deterministic pagination, scrolling,
  active-sprint context, and a backlog column.
- Added ticket detail and help overlays, local filtering, Jira search, status
  transitions, worklogs, issue creation, and sprint membership actions.
- Added stale-response isolation and bounded loading, transition, and modal
  states so asynchronous Jira activity does not corrupt the active board.

### CLI and agent automation

- Added Cobra commands for boards, projects, ticket lookup, JQL search, export,
  transitions, worklogs, creation, comments, assignment, and updates.
- Added curated machine-readable JSON with flattened Atlassian Document Format,
  RFC 3339 dates, browse URLs, and status categories.
- Added schema-valid ADF writes, real Jira mentions, email-based user lookup,
  and active-sprint placement for newly created issues.

### Agent-first access

- Added `gojira mcp`, a stdio MCP server that is read-only by default and
  exposes Jira reads with explicit completeness and mutation-state semantics.
- Added opt-in `--allow-writes` with create, comment, worklog, assign,
  transition, limited update, and delete tools. Writes are never retried
  automatically. A delete confirmation token prevents accidental single-call
  deletion but is not proof of human approval.
- Added project context resolved from a flag, environment variable, local Git
  configuration, or user configuration, with `gojira init` and
  `gojira context --json`. No repository file is created or modified.
- Added `gojira users search` and `gojira report time` (CLI and MCP) for
  per-person worklog totals over an explicit date range, reporting whether the
  visible Jira data is complete.
- Added an optional per-user agent guide (`gojira guide`,
  `gojira guide install`). Nothing is installed unless requested, a
  repository-local guide must be excluded from Git, and existing files are
  never overwritten.

### Configuration and reliability

- Added XDG configuration, OS-keychain token storage with a protected YAML
  fallback, environment-variable overrides, and a first-run TUI wizard.
- Added Jira pagination, transient retry handling for idempotent requests,
  escaped issue paths, encoded JQL, and explicit response decoding errors.
- Added bounded, credential-redacted Jira error details (`httpStatus`,
  `errors`, `errorMessages`) to failed CLI JSON and MCP writes, so a rejected
  change shows its cause without exposing raw responses.
- Added version reporting and cross-platform GoReleaser archives for Linux,
  macOS, and Windows on amd64 and arm64.
