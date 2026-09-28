<p align="center">
  <img src="assets/gojira-logo.png" alt="GoJira logo" width="220"/>
</p>

<h1 align="center">GoJira</h1>

<p align="center">
  <em>Agentic TUI &amp; CLI for Jira boards</em><br/><br/>
  <img src="https://img.shields.io/badge/Go-1.26.1+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License"/>
</p>

<p align="center">
  A Bubble Tea TUI for daily board work and a Cobra CLI with machine-readable
  output for automation and AI agents.
</p>

## Features

- **Kanban board view** — contextual board/sprint/column/page status, active sprint auto-detected, backlog column, horizontal pagination, cursor-following scroll
- **Ticket workflows** — view, search, transition, log work, create, and manage sprint membership in the TUI; the CLI also supports comments, assignment, and field updates
- **Sprint-aware** — create tickets straight into the active sprint, or pull backlog tickets into it (`n` / `a`)
- **Agent-ready JSON** — curated ticket output (ADF flattened to text, RFC3339 dates, browse URLs, `statusCategory`) for AI agents and `jq` pipelines
- **Credential storage** — API token in the OS keychain when available, with a YAML fallback that requests `0600` when the file is created
- **Persistent config** — `~/.config/gojira/config.yaml`, first-run TUI wizard, no `.env` required in production
- **Keyboard-first TUI** — vim-like navigation, copy tickets as JSON (`c`/`C`), open in browser (`o`)

## Installation

> **Not available yet:** GoJira has no published release, so the commands below
> cannot install GoJira until the first release separately publishes the installer,
> archive, and checksum assets. (A shell pipeline may still exit successfully if the download fails.) There is currently no supported no-Go installation route.

After that release is published, Linux and macOS users can install without Go or
a repository checkout:

```bash
curl -fsSL https://github.com/yepizrene-devoost/gojira/releases/latest/download/install.sh | sh
```

The default destination is `$HOME/.local/bin`. The installer does not modify
`PATH`; if that directory is absent from `PATH`, it prints the shell-profile
entry to add. Put overrides on the `sh` side of the pipeline to pin a release or
choose another destination:

```bash
curl -fsSL https://github.com/yepizrene-devoost/gojira/releases/latest/download/install.sh \
  | GOJIRA_VERSION=v1.2.3 GOJIRA_INSTALL_DIR="$HOME/bin" sh
```

After that release is published, Windows users can run this in PowerShell:

```powershell
irm https://github.com/yepizrene-devoost/gojira/releases/latest/download/install.ps1 | iex
```

The Windows installer defaults to `%LOCALAPPDATA%\Programs\gojira` and adds that
directory to the user `PATH`; open a new terminal afterward. Set overrides in
the same PowerShell session before running the one-liner:

```powershell
$env:GOJIRA_VERSION = 'v1.2.3'
$env:GOJIRA_INSTALL_DIR = "$HOME\bin"
$env:GOJIRA_SKIP_PATH_UPDATE = '1'
irm https://github.com/yepizrene-devoost/gojira/releases/latest/download/install.ps1 | iex
```

Omit `GOJIRA_VERSION` to select the latest published release. Remove the override
environment variables afterward if you do not want them to affect later runs.

### Installer trust and integrity

The one-line commands execute remotely downloaded code immediately. Review the
installer URL before running it, or download the script first and inspect the
local file if that trust model is not acceptable.

The installer downloads a GoReleaser archive and `checksums.txt` from the same
GitHub release. SHA-256 verification detects an archive that does not match that
release's checksum entry. It does not independently prove who published either
asset, and it does not remove the need to trust the installer script itself.

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
When creating `config.yaml`, the fallback requests mode `0600`; rewriting an
existing file does not correct a pre-existing broader mode. The YAML is not
encrypted.

Run `gojira` without arguments to see the available commands. To start the
interactive board, invoke the TUI explicitly:

```bash
gojira tui
```

The bare command only prints help; it does not require configuration, open the
setup wizard, or connect to Jira.

## CLI commands

| Command | Description |
|---|---|
| `gojira` | Show available commands and usage |
| `gojira tui` | Launch the interactive TUI (including first-run setup and connection checks) |
| `gojira boards` | List boards with project context (`--json`) |
| `gojira projects` | List Jira projects (`--json`) |
| `gojira get <KEY>` | Full ticket details: project, labels, components, comments (`--json`) |
| `gojira search <JQL>` | JQL search, e.g. `gojira search "assignee = currentUser()"` (`--json`) |
| `gojira export` | Export board/sprint tickets as JSON (`--board`, `--sprint`) |
| `gojira move <KEY> --to <STATUS>` | Transition a ticket (`--json`; no `--to` lists available transitions) |
| `gojira log <KEY> --time 2h` | Add worklog (`--json`, `--comment`, `--show` for history) |
| `gojira create` | Create an issue (`--json`, `--project`, `--type`, `--summary`, `--description-file`, `--board` to place it in the board's active sprint) |
| `gojira comment <KEY> <text>` | Comment with `--json` and optional `--mention email` (resolves to @mention) |
| `gojira assign <KEY> <email>` | Assign by email (`--json`; resolves to accountId) |
| `gojira update <KEY>` | Update `--summary`, `--description`, `--priority`, `--labels` (`--json`; `--description=` clears the description) |
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

# Replace a description using plain text converted to Jira ADF
gojira update ARA-1892 --description $'First paragraph\n- checklist item'
```

JSON modes write one valid JSON document to stdout, with diagnostics on stderr.
The curated `TicketJSON` schema is used by `get --json`, `search --json`,
`export`, TUI ticket/column copy, and successful JSON mutations. `get --json`,
ticket copy, and `create`/`update`/`assign`/`move --to`/`comment`/`log` with
`--json` produce one object; mutation commands re-fetch the full issue after the
write. Search, export, and column copy produce a top-level array. Each ticket
object carries `schemaVersion: "v1"`. Human mutation output is unchanged when
`--json` is omitted.

### TicketJSON v1 contract

All field types are stable JSON types. A field marked **always** is present even
when its string value is empty. An **omitted when empty** field uses JSON
`omitempty`: absent Jira data is represented by no property, not `null` or an
empty array. Jira descriptions and comment bodies are flattened from ADF to
plain text, and recognized Jira timestamps are normalized to RFC3339. Unknown
date formats are preserved verbatim.

| Field | JSON type | Presence and meaning |
|---|---|---|
| `schemaVersion` | string | Always; exactly `"v1"` for this contract. |
| `key` | string | Always; Jira issue key, or an empty string if unavailable. |
| `url` | string | Omitted when empty; HTTPS browse URL, available when both domain and key are known. |
| `summary` | string | Always; empty string when unavailable. |
| `status` | string | Always; status display name, or empty string when unavailable. |
| `statusCategory` | string | Omitted when empty; Jira category key such as `new`, `indeterminate`, or `done`. |
| `priority` | string | Always; priority display name, or empty string when unavailable. |
| `assignee` | string | Always; assignee display name, or `"Unassigned"` when Jira has no assignee. |
| `reporter` | string | Omitted when empty; reporter display name. |
| `issueType` | string | Always; issue type display name, or empty string when unavailable. |
| `project` | string | Omitted when empty; Jira project key. |
| `labels` | array of strings | Omitted when empty; Jira label values. |
| `components` | array of strings | Omitted when empty; component display names. |
| `description` | string | Always; flattened ADF text, or empty string when unavailable. |
| `created` | string | Always; RFC3339 when recognized, empty when unavailable, otherwise the original Jira value. |
| `updated` | string | Always; RFC3339 when recognized, empty when unavailable, otherwise the original Jira value. |
| `timeLogged` | string | Omitted when empty; reserved curated time-log summary when supplied by a read surface. |
| `comments` | array of objects | Omitted when empty; comments in Jira order. Each object always has string `author`, flattened string `body`, and string `created` with the same date rules. |

Read surfaces can only populate fields requested and returned by their Jira
endpoint. In particular, board-backed export and TUI column copy request basic
issue fields, so description, comments, and timestamps may be empty or omitted.
Sparse Jira responses preserve the required empty-string fields and the
`"Unassigned"` default described above; they never synthesize `null` values.

`boards --json` and `projects --json` expose their own list shapes.
`move --json` without `--to` retains its transition array, and
`log --json --show` retains its worklog array. `version --json` has its own
`version`/`revision`/`dirty` schema.

A failed JSON mutation exits with status 1, writes no stdout before result
encoding begins, and writes one error object to stderr:

```json
{
  "schemaVersion": "v1",
  "code": "mutation_failed",
  "message": "issue update failed",
  "issueKey": "ARA-1892",
  "mutationState": "unknown"
}
```

`code` and `message` are stable, non-sensitive summaries; `issueKey` is omitted
when it is not known. `mutationState` is exactly `applied`, `not_applied`, or
`unknown`. A successful write followed by a failed re-fetch is `applied`; a
confirmed HTTP 4xx rejection is `not_applied`; transport failures, HTTP 5xx,
and refused HTTP redirects after dispatch are `unknown`. Writes are sent once,
never retried, and 307/308 redirects are not followed. If issue creation
succeeds but sprint placement fails, the error includes the created key and
`applied`. An output writer may fail after a partial stdout write, so
stdout atomicity cannot be guaranteed in that case; the command still returns
failure and never reports success.

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
Transition, issue creation, and worklog entry remain full-screen flows. The
transition flow shows bounded progress while loading choices and submitting a
move; repeated keys cannot start duplicate requests. `Esc` explicitly dismisses
pending transition UI and ignores its late reply, but cannot cancel an HTTP
mutation already in flight, which may still complete in Jira. If a same-board
sprint refresh dismisses the picker instead, the mutation remains owned: success
shows a toast and starts a fresh board refresh, while failure remains visible
after the sprint refresh. Switching boards rejects the old board's response.

The Kanban status always identifies the board, sprint, selected column, and
horizontal page. On narrower terminals it gives board and sprint separate space
alongside compact column/page context, even when names are long or the shortcut
footer collapses.

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
| `e` | Edit the description; `Ctrl+S` confirms and `Esc` cancels without writing |
| `w` | Add worklog (cancel or success returns to this detail modal) |
| `o` | Open in browser |
| `Esc` | Back to the board or search results |

The description editor accepts multiple lines and converts an actual plain-text
edit to Jira ADF. Replacing a description this way may discard rich ADF formatting
that plain text cannot represent; confirming unchanged text performs no write. A
successful update refreshes the open ticket detail. If the update succeeds but
refresh fails, the editor marks the update as already applied and `Ctrl+S` retries
only the detail refresh; closing that state does not roll back the Jira update.
Other failures keep the editor text and ticket context available for retry or
cancel.

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
├── scripts/                   # Linux/macOS and Windows release installers
├── tests/                     # Installer contracts and Go test support
├── assets/                    # Logo (PNG + SVG)
└── .github/                   # CI, release checks, and issue forms
```

## API surface

- **Platform REST API v3** (`/rest/api/3/*`) — issues, transitions, worklogs, comments, users, projects
- **Agile REST API 1.0** (`/rest/agile/1.0/*`) — boards, board configuration, sprints

All endpoints live in `internal/jira/paths.go`. If Atlassian deprecates a path, that file is the only place to change.

## Development

Source development requires Go 1.26.1 or newer. From an existing source tree:

```bash
go build -o gojira .
./gojira --help
./gojira version --json
go test ./...
make lint
```

Ordinary source builds generally report `dev` as their version; release builds
receive the release tag. The version JSON also reports the embedded Git revision
and whether the source tree was dirty when built.

Run `make build-all` for optional local validation of all six release platforms:
Linux, macOS, and Windows on amd64 and arm64. The cross-build uses
`CGO_ENABLED=0`, writes explicitly named binaries under the overridable
`BIN_DIR` (`bin/` by default), and adds `.exe` to Windows binaries. It consumes
more CPU and disk than a single build and overwrites those six output names, but
preserves other files in the directory. The default `bin/` directory is ignored
by Git.

Unlike `make release`, `make build-all` does not read `.env`, create tags,
or invoke GoReleaser to publish anything. Go may still download missing modules
or toolchains during a local build; run with `GOPROXY=off GOTOOLCHAIN=local` if
network access must be prohibited. The installer one-liners above remain
unavailable until the first release is published separately.

`make lint` requires an already installed `golangci-lint` v2.11.4 and fails on a
missing or different version; it does not install tools. The lint command covers
every Go package through `./...`, while non-Go files need separate checks. CI
builds, tests, vets, and lints the Go code. Separate release checks validate the
GoReleaser configuration, POSIX and Windows installer contracts, and curated
release-note extraction. Making any job a required check is separate branch
protection configuration.

Run the TUI in a real terminal (it needs a TTY):

```bash
./gojira
```

### Release maintenance

Release notes are curated; generated history is only reference material.

| Target | Result |
|---|---|
| `make changelog` | Writes a heading-free, grouped commit draft to `CHANGELOG.draft.md`; `docs(odd)` bookkeeping is omitted. |
| `make release-notes VERSION=vX.Y.Z` | Extracts the body of the newest matching package section from `CHANGELOG.md` into ignored `RELEASE_NOTES.md`. |
| `make release` | Loads `GITHUB_TOKEN` from `.env`, requires an exact local version tag, regenerates the notes, and publishes or replaces the GitHub release through GoReleaser. |

`make release` is the repository's only publisher and is an explicit local
publishing command, not a dry run. GitHub Actions validates more than the
GoReleaser configuration, but it does not publish; pushing a tag does not start
a CI publisher. Maintainers must follow [RELEASING.md](RELEASING.md) for the
dflow promotion, tag-on-`main`, token permission, approval, rerun, and
verification checklists.

## License

MIT License — see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome. Use the issue templates (bug / feature / chore) and open a PR. Maintainers should follow [RELEASING.md](RELEASING.md) for release promotion, local publication, and artifact verification.
