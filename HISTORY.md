# Project history

This document records the development arc behind each published GoJira release.
For concise user-facing changes, see [CHANGELOG.md](CHANGELOG.md).

## v0.1.0 — From board viewer to daily Jira client

GoJira's first release was built over four days of merged development. The work
started with a terminal board and expanded into a Jira client designed for both
people and automation.

### September 24, 2026 — Initial foundation

The repository began with the core Go application and its Jira integration. This
established the base that later work split into CLI, configuration, Jira-domain,
and terminal-interface packages.

### September 25, 2026 — Complete interactive and scripted workflows

The first development wave made the Kanban board predictable under real terminal
constraints by adding deterministic layout, horizontal pagination, and scrolling.
Clipboard actions exported curated ticket JSON, while transitions and worklogs
made the interface useful for daily ticket maintenance rather than viewing alone.

In parallel, the application gained a Cobra CLI for scripting and agent use. Read
commands were enriched with stable, useful fields, and Jira's Atlassian Document
Format was flattened at the domain boundary. Write commands then covered issue
creation, comments, assignment, updates, transitions, and worklogs. Issue creation
and sprint membership were added to the TUI, including direct placement into an
active sprint.

Configuration moved to the XDG config directory with token storage in the OS
keychain and a protected YAML fallback. A first-run wizard made that setup
available from the TUI. Follow-up fixes corrected wizard flow, board project
names, endpoint ownership, ADF writes, mentions, user lookup, and pagination of
board and sprint issue loading.

The same day also established release basics: version reporting, build targets,
a git-cliff draft target, the MIT license, repository issue templates, and the
project's logo and user documentation.

### September 26, 2026 — Reliability and distribution

A focused reliability pass made network and credential failures explicit. Jira
requests gained encoded JQL, escaped issue paths, transient retries for safe
idempotent operations, response-body error coverage, and protection against
retrying non-idempotent writes. Configuration tests covered token storage and
invalid input paths.

Release engineering added CI validation for version output and GoReleaser-based
cross-platform distribution. The resulting release matrix covers Linux, macOS,
and Windows on amd64 and arm64, with checksums and platform-appropriate archives.

The terminal interface then moved to the current Bubble Tea generation. Board
loading became isolated from stale asynchronous responses, and ticket details and
help became overlays that preserve the underlying board context.

### September 27, 2026 — Search and interaction polish

The final merged work before v0.1.0 bounded overlay geometry for small terminals,
added immediate local filtering and Jira-backed search, and exposed clearer board,
sprint, column, page, and transition activity. These changes completed the first
release as a keyboard-first Jira workflow for interactive use and agent-oriented
automation.
