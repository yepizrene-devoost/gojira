# Changelog

GoJira release notes are curated from changes merged into the project. The
heading of the newest package section is also the source for local release
publishing.

## 📦 Unreleased – Update-flow fixes

### Update-flow fixes

- The update check now truly runs on every TUI start by default. The interval
  is read from the configured `update_check_interval` (absent, `0`, unparsable
  or negative means every start); a fresh install checks on every start too,
  because nothing was recorded yet. Previously the launch wiring passed a
  hardcoded 24-hour constant and the configured value was never consulted.
- After a successful install, a final modal offers to restart gojira right away
  (`r`) — the process is replaced with the new binary in place — or `Later`,
  which keeps the previous manual-restart notice. A failed replacement falls
  back to that notice and is never retried.

## 📦 v0.8.1 – Comment management

### Comment management

- Comments can now be managed, not only created, from every surface. The CLI
  lists the complete paginated comment history (`gojira comment KEY --list`,
  `--json` with `total`), reads one comment by its stable id (`--id ID`), edits
  its body (`--id ID --text "..."`) and deletes it with the echoed id
  (`--id ID --delete --confirm ID`). The existing add-comment behavior is
  unchanged.
- Editing replaces the whole comment body with plain text as ADF: rich
  formatting, mentions and visibility restrictions of the previous body are
  lost. Every surface states this before the replacement — the CLI prints a
  `warning:` on stderr (never on `--json` stdout), the TUI editor states it in
  its header and the MCP tool description documents it.
- MCP agents get a complete comment history read (`gojira_comments`, unlike
  the partial embedded comments of `gojira_get`) and two write tools:
  `update_comment` and `delete_comment`. Deleting stays a two-step, single-use
  token flow bound to `action:key:commentId`; a worklog token cannot delete a
  comment. Outcomes keep the applied / not_applied / unknown reporting and
  never retry automatically.
- The TUI gains a comment management view from the ticket detail (`C`): the
  full collection loads from the comment endpoint (not the embedded page),
  `↑`/`↓` select, `e` opens a prefilled editor saved once with `Ctrl+S`, and
  `d` + `y` deletes with a visible preview. A success reloads the list and
  refetches the ticket; the ADF replacement warning stays visible.
- Comment ids are additive in the TicketJSON `comments` array, matching the
  worklog `id` precedent; no existing field changed.

## 📦 v0.8.0 – Update prompt

### Update prompt

- The TUI now checks for a newer release on every start instead of once a day.
  The check sends the stored `ETag` as `If-None-Match`, so an unchanged release
  answers `304` and does not consume GitHub's rate limit; releases published the
  same day are no longer invisible. `update_check_interval` (a duration; absent
  or `0` means every start) spaces checks out when wanted, `update_check false`
  disables them, and offline use, failures and development builds stay silent.
- When a newer release exists, a centred modal offers **Update now**, **View
  changes** and **Keep current version**, with the cursor starting on Keep.
  `u`, `v` and `c` are direct shortcuts, and a dismissed or viewed version never
  prompts again. A status line in the boards header stays until the running
  version is current.
- **Update now installs from the TUI**: the same verified installer the CLI uses
  — ownership is checked before any network access, the download is verified
  against the release checksums, and placement stays atomic. Progress and every
  refusal render inside the modal, and an installation in flight cannot be
  cancelled. The running session keeps the old binary until it is restarted.
- This reverses the v0.6.0 statement that the TUI never installs anything. The
  decision was taken deliberately for convenience; every installer guard is
  unchanged.

## 📦 v0.7.0 – Worklog management

### Worklog management

- Worklogs can be corrected and removed instead of only created:
  `gojira log KEY --id ID` updates the duration and the comment, `--clear-comment`
  removes the comment, and `--delete --confirm ID` removes the entry after the id
  is repeated. Every worklog now carries Jira's `id`, in `log --show --json` and
  in the ticket `worklogs` array.
- Agents get the same reach through the MCP tools `update_worklog` and
  `delete_worklog`. Deletion reuses the single-use confirmation token, expires
  after two minutes, and is bound to that exact worklog so a token issued for one
  target cannot redeem on another.
- The TUI lists worklogs with `W`, reading the complete collection from Jira
  instead of the shorter page embedded in the ticket read. `e` reopens an entry
  prefilled and saves it once with `Ctrl+S`; `d` deletes only after `y`.

### Worklog comments

- Creating or updating a worklog with a comment now succeeds: the comment travels
  as an ADF document, which is what Jira requires. It previously carried an empty
  plain string and Jira rejected the whole worklog with HTTP 400.
- Saving an emptied comment removes it, and a duration change in the same save
  travels in one request, so a failure cannot leave the entry half-edited.
- The comment is plain text converted to ADF, so it cannot reproduce richer
  formatting.

### Post-write verification

- After an update the entry is re-read. When Jira kept a duration other than the
  requested one, the CLI prints a warning on stderr without changing its exit
  status, the TUI shows it beside the success toast, and the MCP result carries it
  in `hint` while the state stays applied.
- This exists because an entry owned by another app can accept the write and keep
  its own value in that app's views.

### Release tooling

- `make release` now requests the published tag from the public Go module proxy
  after GoReleaser, so `go install …@latest` and `pkg.go.dev` can resolve it
  without waiting for an unrelated request. `make warm-module-proxy VERSION=vX.Y.Z`
  is the recovery path, and a failed warm-up reports that the release may already
  exist instead of claiming the publish failed.

### Notes

- Clearing a comment sends a null comment: the shape is covered by the release
  tests, but it was not verified against a live Jira site when this version was
  prepared.
- Worklogs owned by Tempo keep their own records, so editing them through Jira
  does not update Tempo's views.
- `log KEY --show --id ID` is rejected on purpose: reading the history and
  updating a single entry are separate actions.

## 📦 v0.6.0 – Safe self-upgrade

### Upgrade command

- `gojira upgrade` replaces an installer-owned binary with the latest published
  release. `--check` reports without installing, `--json` emits one `v1`
  document on stdout, and `--yes` is required when no terminal is attached.
  `gojira update` still updates a Jira issue and is unchanged.
- Only `$GOJIRA_INSTALL_DIR`, `~/.local/bin`, and on Windows
  `%LOCALAPPDATA%\Programs\gojira` are replaced. Package-manager paths,
  `go install` locations, symlinked executables or install directories, and
  development builds are refused with instructions instead of being overwritten.
- The archive is verified against `checksums.txt` from the same release before
  anything is staged, with size caps on the download and on the extracted binary,
  and rejection of traversal paths, absolute paths, drive letters, symlinks,
  hardlinks and duplicate binaries. A matching checksum proves transfer integrity
  only: it is published next to the artifact, so it does not authenticate the
  publisher.
- Replacement is staged at mode 0755 and moved atomically on Unix. On Windows the
  previous binary is moved aside so it is never lost, a failed placement is rolled
  back, and a blocked double rename reports `unknown` with the leftover path to
  verify. An exclusive lock prevents concurrent upgrades, and nothing is retried
  automatically.

### Update prompt

- The TUI checks on every start for a newer release using a conditional request
  (`If-None-Match`): an unchanged release answers `304` without consuming rate
  limit, and `update_check_interval` can still space checks out (`0` or absent
  means every start). A centred modal offers Update now, View changes, and Keep
  current version, with the cursor on Keep.
- Update now runs the same verified installer path as the CLI — ownership first,
  then the checksum-verified download and atomic placement — and shows progress
  and every refusal inside the modal. The running session keeps the old binary
  until it is restarted. This reverses the v0.6.0 statement that the TUI never
  installs anything; that decision was taken deliberately for convenience, and
  every installer guard is unchanged.

### Update notification

- The TUI checks on every start for a newer published release with a four second
  timeout, using a conditional request (`If-None-Match`) so an unchanged release
  does not consume rate limit; `update_check_interval` spaces checks out when
  configured, and failures, offline use and development builds stay silent. When
  a newer release exists, a centred modal offers Update now, View changes, and
  Keep current version. Update now runs the same installer the CLI uses, with
  every guard intact, and reports refusals inside the modal; the running session
  keeps the old version until it is restarted. (Since v0.7.0 the TUI can install
  updates; the earlier statement that it never installs anything no longer
  holds.)
- The check is controlled with `gojira config set update_check false` and read
  with `gojira config get update_check`.

### Notes

- A development build refuses to replace itself; install a published release with
  the installer scripts first.
- Windows replacement was verified through an injected platform seam on Linux,
  not on a Windows host.

## 📦 v0.5.0 – Attachments and interface parity

### Attachments

- Upload local files to Jira issues from the CLI (`gojira attach`), the opt-in
  MCP write tool (`gojira_attach`), or the TUI ticket detail (`f`).
- Uploads check Jira's attachment settings and file size before sending, use one
  request per file, and do not automatically retry writes. The MCP tool can read
  any regular file accessible to its server process; enable writes only for
  trusted workflows.

### Interface parity

- MCP can create subtasks with a parent, assign issues by email, create issues
  in an active sprint, and export board or sprint tickets as curated JSON.
- The TUI supports parent selection for subtasks, Jira user search, configurable
  search limits and JQL search, time reports, and copying board or sprint exports
  to the clipboard.

### Notes

- TUI uploads cannot be cancelled after submission. If the outcome is unknown,
  check Jira before retrying to avoid duplicate attachments.

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
