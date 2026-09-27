# Changelog

GoJira release notes are curated from changes merged into the project. The
heading of the newest package section is also the source for local release
publishing.

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

### Configuration and reliability

- Added XDG configuration, OS-keychain token storage with a protected YAML
  fallback, environment-variable overrides, and a first-run TUI wizard.
- Added Jira pagination, transient retry handling for idempotent requests,
  escaped issue paths, encoded JQL, and explicit response decoding errors.
- Added version reporting and cross-platform GoReleaser archives for Linux,
  macOS, and Windows on amd64 and arm64.
