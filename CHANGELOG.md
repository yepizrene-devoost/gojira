# Changelog

GoJira release notes are curated from changes merged into the project. The
heading of the newest package section is also the source for local release
publishing.

## 📦 v0.4.0 – TUI refinements and search by issue key

### TUI

- Ticket cards are easier to scan: the selected card gets an accent bar on
  every line and one uniform background (a nested style reset used to cut the
  fill short), priority shows as a traffic-light bullet whose shape also
  encodes the level (`●●` Highest, `●` High, `○` Low, `○○` Lowest; Medium shows
  nothing), and the assignee appears with initials.
- The active Kanban column carries a `▌` focus marker, visible even when it is
  empty, with an `(empty column)` placeholder. `Enter` on an empty column now
  says `Column is empty` instead of doing nothing.
- Board, board-list and search waits show an indeterminate spinner that stops
  when nothing is loading.
- The board list opens under a rounded, width-bounded frame with the wordmark
  and the build version, centered in the terminal. Terminals that are too small
  keep the one-line title.
- Accents use the colors of the app logo (cyan, deep blue, orange and green)
  instead of purple.

### Search

- A query shaped like an issue key (for example `mon-201`) looks that issue up
  by key and falls back to the text index when Jira rejects the lookup, so
  typing a key in the TUI search finds the ticket.
- The TUI search header reports completeness honestly: `N shown`,
  `N shown · ≥M estimated`, or `N shown · more results`. It used to print
  `0 total` for truncated results because the estimate was discarded.

### Fixes

- The board list shortcut hint and board names no longer overflow narrow
  terminals; they are truncated to the available width.

### Internal

- `Client.SearchText` and `Client.SearchJQL` now return the full `SearchResult`
  (including `Truncated` and `EstimatedTotal`). `cmd search` and the MCP search
  tool are unchanged because they already used `SearchIssues`.

## 📦 v0.3.0 – Parity units: shared writes, sprint lifecycle and TUI field editor

### Multi-surface parity

All three surfaces (CLI, MCP, TUI) share the same business rules for update,
sprint lifecycle and field editing. The parity audit (issue #30) drives these
changes: code is shared, surface-specific adapters handle flags, structured
tools and visual forms separately.

#### Shared update fields

- The MCP tool `gojira_update` now uses the same field validation, preparation and
  error messaging that the CLI `update` command builds, so both surfaces produce
  identical change-only payloads, reject the same invalid inputs and surface
  actionable messages on validation failure.
- MCP now permits description replacement (plain text sent as valid ADF),
  superseding the previous prohibition. Rich-ADF formatting is still lost on
  update; see the integration guide.
- Story points can be set or cleared through a paired field-ID lookup rather
  than a separate tool call.

#### Sprint lifecycle writes

- New MCP write tools: `gojira_sprint_move` (add issues to a sprint),
  `gojira_backlog` (move issues to backlog), `gojira_close` and
  `gojira_reopen` (transition by status category or exact transition).
- Shared `internal/lifecycle.Select`, `Close`, `Reopen` – the CLI and MCP
  validate transitions once, use the same selection UI, and the CLI retains
  its existing output format.
- Sprint moves require an active-sprint board ID or the user supplies it
  explicitly.

#### TUI field editor

- `U` in the ticket detail opens a bounded form to edit summary, priority and
  labels. Tab/Shift-Tab moves focus, paste injects text, `Ctrl+S` saves, `Esc`
  cancels.
- Only changed fields are sent; empty summary, empty priority and cleared
  labels are rejected inline with zero requests (matching CLI/MCP constraints).
- On success the open detail and the board card/search row refresh with the
  new values. On failure the form stays with the typed text so another save
  is possible. A failed refresh after an applied update retries only the
  refresh.
- `ParseLabelList` in `internal/issuefields` trims comma-separated input,
  preserves order/case and rejects empty entries.

### Release notes

- Changelog entries for v0.2.0 are preserved below.
- Signed release tags: all v0.x tags are PGP signed annotated tags.

## 📦 v0.2.0 – Safer onboarding and MCP setup

### Configuration

- Mask typed and pasted API tokens with asterisks in the `gojira config init`
  terminal prompt. Esc or Ctrl+C cancels the prompt without saving.
- Add `--token-stdin` for non-interactive token input, with explicit `--domain`
  and `--email`. Existing line-based input without a terminal remains supported.
- Keep `--token` compatible but warn that command-line arguments can expose
  credentials through shell history and process lists.

### Agent integration

- Add `gojira mcp config --client codex|json` to print ready-to-paste client
  configuration using the running executable's absolute path, without credentials
  or file writes. Output is read-only by default; `--allow-writes` is explicit.

### Documentation

- Add linked English and Spanish integration guides covering API tokens,
  local configuration, identity checks, and CLI, TUI, and MCP onboarding.
- Update the README and both guides for masked input and `--token-stdin`.
- Clarify that `config init` and `config test` check site connectivity, not
  credential validity. Use `gojira context` to verify the effective identity.

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
