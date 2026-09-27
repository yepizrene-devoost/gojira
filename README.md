<p align="center">
  <img src="assets/gojira-logo.png" alt="GoJira logo" width="220"/>
</p>

<h1 align="center">GoJira</h1>

<p align="center">
  <em>Agentic TUI &amp; CLI for Jira boards</em><br/><br/>
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License"/>
</p>

<p align="center">
  A Bubble Tea TUI for daily board work and a Cobra CLI whose JSON output is
  built for AI agents — every operation available both interactively and
  programmatically.
</p>

## Features

- **Kanban board view** — columns, active sprint auto-detected, backlog column, horizontal pagination, cursor-following scroll
- **Full ticket management** — view, transition status, add worklog, comment, create, assign, update — in CLI *and* TUI
- **Sprint-aware** — create tickets straight into the active sprint, or pull backlog tickets into it (`n` / `a`)
- **Agent-ready JSON** — curated output (ADF flattened to text, RFC3339 dates, browse URLs, `statusCategory`) for AI agents and `jq` pipelines
- **Secure auth** — API token in OS keychain (macOS/Linux/Windows) with encrypted-file fallback
- **Persistent config** — `~/.config/gojira/config.yaml`, first-run TUI wizard, no `.env` required in production
- **Keyboard-first TUI** — vim-like navigation, copy tickets as JSON (`c`/`C`), open in browser (`o`)

## Installation

Download the archive for your platform from [GitHub Releases](https://github.com/yepizrene-devoost/gojira/releases), verify it with `checksums.txt`, and place the `gojira` binary on your `PATH`.

You can also install or build from source:

```bash
go install github.com/yepizrene-devoost/gojira@latest

git clone https://github.com/yepizrene-devoost/gojira.git
cd gojira
go build -o gojira .
```

Confirm the installed release and source revision:

```bash
gojira version --json
```

## Getting your Jira API token

1. Go to [Atlassian API Tokens](https://id.atlassian.com/manage-profile/security/api-tokens)
2. Click **Create API token**
3. Name it (e.g. "GoJira") and copy it

## Configuration

Run the setup wizard once — it stores your domain and email in `~/.config/gojira/config.yaml` and your token in the OS keychain:

```bash
gojira config init
```

Or non-interactively:

```bash
gojira config init --domain mycompany.atlassian.net --email me@example.com --token <API_TOKEN>
```

Environment variables still work and take precedence over the config file (useful for CI):

```bash
export JIRA_EMAIL="you@example.com"
export JIRA_API_TOKEN="..."
export JIRA_DOMAIN="your-domain.atlassian.net"
```

Credential resolution: **env vars > config file (keychain → YAML fallback)**.

## CLI commands

| Command | Description |
|---|---|
| `gojira` | Launch the interactive TUI |
| `gojira boards` | List boards with project context (`--json`) |
| `gojira projects` | List Jira projects (`--json`) |
| `gojira get <KEY>` | Full ticket details: project, labels, components, comments (`--json`) |
| `gojira search <JQL>` | JQL search, e.g. `gojira search "assignee = currentUser()"` (`--json`) |
| `gojira export` | Export board/sprint tickets as JSON (`--board`, `--sprint`) |
| `gojira move <KEY> --to <STATUS>` | Transition a ticket (no `--to` lists available transitions) |
| `gojira log <KEY> --time 2h` | Add worklog (`--comment`, `--show` for history) |
| `gojira create` | Create an issue (`--project`, `--type`, `--summary`, `--description-file`, `--board` to place it in the board's active sprint) |
| `gojira comment <KEY> <text>` | Comment with `--mention email` (resolves to @mention) |
| `gojira assign <KEY> <email>` | Assign by email (resolves to accountId) |
| `gojira update <KEY>` | Update `--summary`, `--priority`, `--labels` |
| `gojira config` | Manage configuration: `init`, `set`, `get`, `path`, `test` |
| `gojira version` | Version marker + build revision (`--json`) |

### Agent usage examples

```bash
# Feed a sprint to an AI agent
gojira export --board 1 | jq '.[] | select(.assignee == "René")'

# What can this ticket become? (transition picker as JSON)
gojira move ARA-1892 --json

# Full requirement context as JSON
gojira get ARA-1892 --json

# Create a ticket from a markdown requirement file
gojira create --project ARA --type Task --summary "Add export" --description-file req.md

# Create a ticket straight into the active sprint (skip the backlog)
gojira create --project ARA --type Task --summary "Sprint work" --board 1

# Comment and notify the PM
gojira comment ARA-1892 "Ready for review" --mention pm@devoost.com
```

All `--json` output is a stable, machine-readable contract: valid JSON on stdout, diagnostics on stderr.

## TUI keyboard shortcuts

### Boards list

| Key | Action |
|---|---|
| `↑/↓` or `k/j` | Navigate boards |
| `Enter` | Open board |
| `s` | Search all Jira tickets |
| `q` | Quit |

### Kanban view

| Key | Action |
|---|---|
| `←/→` or `h/l` | Move between columns (pages horizontally) |
| `↑/↓` or `k/j` | Move within column (scrolls with cursor) |
| `Enter` | Open ticket detail in a floating modal |
| `/` | Filter loaded tickets by key or summary (case-insensitive); `Enter` searches Jira for the same text |
| `s` | Search all Jira tickets directly |
| `?` | Show the Kanban help overlay (`↑/↓` scrolls; `?`, `Esc`, or `q` closes it) |
| `t` | Transition ticket (status picker) |
| `n` | New issue in this board's project (type + summary + description) |
| `a` | Add selected ticket to the active sprint (e.g. from the Backlog column) |
| `C` | Copy column as JSON to clipboard |
| `Esc` / `q` | Back to boards |
| `Ctrl+C` | Quit, including while an overlay is open |

Ticket detail and help float over the board at a consistent height (up to 35 rows)
and width (up to 100 columns), so long content scrolls inside instead of stretching
the modal. Detail shows the ticket key and summary together in its header; a long
summary also remains available in the scrollable body. The current column
and ticket remain selected after closing them. Overlay keys are isolated from
the board: detail navigation scrolls its content without moving the board cursor.
Transition, issue creation, and worklog entry remain full-screen flows.

Boards with an active sprint show a trailing **Backlog** column with the
unsprinted tickets, so nothing silently disappears from the board. New issues
created with `n` go directly into the active sprint.

Local `/` filtering updates the loaded board immediately. Press `Esc` to clear
it, or press `Enter` with a nonempty filter to search Jira's text index. Search
results support `↑/↓` (or `k/j`) and `Enter` for full ticket detail; closing the
detail returns to the same query, result list, and cursor. Press `s` from a
result list to edit the query, or `Esc` to return to the board or boards list
where search started.

### Ticket detail

| Key | Action |
|---|---|
| `↑/↓` or `k/j`, `PgUp/PgDn` | Scroll long ticket details within the modal |
| `c` | Copy ticket as JSON to clipboard |
| `w` | Add worklog (cancel or success returns to this detail modal) |
| `o` | Open in browser |
| `Esc` | Back to the board or search results |

## Project structure

```
gojira/
├── main.go                    # Entry point
├── cmd/                       # Cobra CLI commands
│   ├── root.go                # Root command + credential resolution
│   ├── tui.go                 # TUI launcher + first-run wizard
│   ├── boards.go projects.go get.go search.go export.go
│   ├── move.go log.go create.go comment.go assign.go update.go
│   └── config.go              # config init/set/get/path/test
├── internal/
│   ├── jira/                  # Jira domain
│   │   ├── client.go          # HTTP client (GET/POST/PUT)
│   │   ├── paths.go           # ALL API endpoints — single source of truth
│   │   └── types.go           # Types + ADF parser + curated TicketJSON
│   ├── tui/                   # Bubble Tea TUI (single model, no sub-model routing)
│   │   ├── tui.go             # Model: boards, kanban, detail, transition, worklog, create, sprint
│   │   ├── setup.go           # First-run configuration wizard
│   │   └── tui_test.go        # Render geometry tests
│   └── config/                # XDG config + keychain token storage
│       └── config.go
├── assets/                    # Logo (PNG + SVG)
└── .github/ISSUE_TEMPLATE/    # Issue forms (bug, feature, chore)
```

## API surface

- **Platform REST API v3** (`/rest/api/3/*`) — issues, transitions, worklogs, comments, users, projects
- **Agile REST API 1.0** (`/rest/agile/1.0/*`) — boards, board configuration, sprints

All endpoints live in `internal/jira/paths.go`. If Atlassian deprecates a path, that file is the only place to change.

## Development

```bash
go build -o gojira .
go test ./...
```

Run the TUI in a real terminal (it needs a TTY):

```bash
./gojira
```

## License

MIT License — see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome. Use the issue templates (bug / feature / chore) and open a PR. Maintainers should follow [RELEASING.md](RELEASING.md) for release promotion, dry-run, tagging, and artifact verification.
