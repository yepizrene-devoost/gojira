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

<p align="center">
  <strong>New here?</strong> Follow the integration guide — from an Atlassian API token to
  your first query and your AI client:
  <a href="docs/integration-guide.md">English</a> ·
  <a href="docs/integration-guide.es.md">Español</a>
</p>

## Features

- **Kanban board view** — contextual board/sprint/column/page status, active sprint auto-detected, backlog column, horizontal pagination, cursor-following scroll
  The TUI Backlog column uses Jira's backlog endpoint plus future-sprint issues.
  Jira documents this endpoint as deprecated; if it fails, the app falls back to
  computing the column from all board issues.
- **Ticket workflows** — view, search, transition, close/reopen, delete, log work, comment, create, and manage sprint membership in both the CLI and the TUI; the CLI also supports assignment and field updates
- **Sprint-aware** — create tickets straight into the active sprint, or pull backlog tickets into it (`n` / `a`)
- **Agent-ready JSON** — curated ticket output (ADF flattened to text, RFC3339 dates, browse URLs, `statusCategory`) for AI agents and `jq` pipelines
- **Credential storage** — API token in the OS keychain when available, with a YAML fallback that requests `0600` when the file is created
- **Persistent config** — `~/.config/gojira/config.yaml`, first-run TUI wizard, no `.env` required in production
- **Keyboard-first TUI** — vim-like navigation, copy tickets as JSON (`c`/`C`), open in browser (`o`)

## Installation

GoJira publishes release archives for Linux, macOS, and Windows on amd64 and
arm64, with a `checksums.txt` and installer scripts, on the
[releases page](https://github.com/yepizrene-devoost/gojira/releases). The first
public release is `v0.1.0`. No Go toolchain or repository checkout is needed.

Linux and macOS users can install with:

```bash
curl -fsSL https://github.com/yepizrene-devoost/gojira/releases/latest/download/install.sh | sh
```

The default destination is `$HOME/.local/bin`. The installer does not modify
`PATH`; if that directory is absent from `PATH`, it prints the shell-profile
entry to add. Put overrides on the `sh` side of the pipeline to pin a release or
choose another destination:

```bash
curl -fsSL https://github.com/yepizrene-devoost/gojira/releases/latest/download/install.sh \
  | GOJIRA_VERSION=v0.1.0 GOJIRA_INSTALL_DIR="$HOME/bin" sh
```

Windows users can run this in PowerShell:

```powershell
irm https://github.com/yepizrene-devoost/gojira/releases/latest/download/install.ps1 | iex
```

The Windows installer defaults to `%LOCALAPPDATA%\Programs\gojira` and adds that
directory to the user `PATH`; open a new terminal afterward. Set overrides in
the same PowerShell session before running the one-liner:

```powershell
$env:GOJIRA_VERSION = 'v0.1.0'
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
2. Click **Create API token** (the plain one, not "with scopes": GoJira calls your site
   address, and Atlassian documents a different base address for scoped tokens)
3. Name it (e.g. "GoJira"), pick an expiration date (1 to 365 days; Atlassian defaults to
   one year) and copy the token: Atlassian shows it only once

When a token expires, create a new one and run `gojira config init` again. The
step-by-step guide, with troubleshooting and AI-client setup, is the
[integration guide](docs/integration-guide.md) ([Español](docs/integration-guide.es.md)).

## Configuration

Run the setup wizard once — it stores your domain and email in `~/.config/gojira/config.yaml` and your token in the OS keychain:

```bash
gojira config init
```

On a terminal the token prompt is masked: what you type or paste appears as `*` and is
never displayed (Esc or Ctrl+C cancels without saving). Without a terminal (pipes,
scripts) it reads one line as before.

To configure without the prompt, pipe the token in instead of passing it as an argument
(`--domain` and `--email` are then required):

```bash
pbpaste | gojira config init --domain mycompany.atlassian.net --email me@example.com --token-stdin
```

`pbpaste` prints the clipboard on macOS; use `Get-Clipboard` in PowerShell, `wl-paste` or
`xclip -o -selection clipboard` on Linux, or the command line of your password manager.
`--token <API_TOKEN>` still works but is discouraged: it stays in your shell history and
in the process list, and GoJira prints a warning when you use it.

`config init` ends with a connection line, and `gojira config test` repeats that check,
but both only prove that the site answers (Jira's server-info endpoint needs no
credentials). To confirm the email and token are valid, run `gojira context`: it asks
Jira who you are.

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
| `gojira init` | Remember the project for this clone: picker with a terminal, `--project KEY --yes` without one (`--json`) |
| `gojira context` | Show the resolved project, its source and the authenticated identity (`--project`, `--json`) |
| `gojira get <KEY>` | Full ticket details: project, labels, components, fix versions, due date, comments, parent and subtasks when returned (`--json`); add `--story-points-field customfield_N` to include that numeric field |
| `gojira search <JQL>` | JQL search, e.g. `gojira search "assignee = currentUser()"` (`--json`, `--limit`, `--all`, `--meta`) |
| `gojira export` | Export board/sprint tickets as JSON (`--board`, `--sprint`) |
| `gojira move <KEY> --to <STATUS>` | Transition a ticket (`--json`; no `--to` lists available transitions) |
| `gojira close KEY` / `gojira reopen KEY` | Select the single available transition into Done / New or In Progress categories (`--json`); ambiguous or missing candidates produce no write |
| `gojira delete KEY --confirm KEY` | Permanently delete one issue after exact key confirmation (`--json` returns `{schemaVersion,key,deleted:true}`) |
| `gojira sprint list --board N` | List sprints (default `--state active,future`; also accepts `closed`); `--json` returns an array of `{id,name,state}`, empty `[]` |
| `gojira sprint move KEY --to ID` | Ask Jira to move one issue into a sprint; `--json` re-fetches TicketJSON v1 |
| `gojira backlog <KEY>` | Ask Jira to move one issue from future and active sprints to the backlog (`--json`) |
| `gojira log <KEY> --time 2h` | Add worklog (`--json`, `--comment`, `--show` for history) |
| `gojira create` | Create an issue (`--json`, `--project`, `--type`, `--summary`, `--description-file`, `--parent` for a sub-task, `--board` for active sprint placement) |
| `gojira comment <KEY> <text>` | Comment with `--json` and optional `--mention email` (resolves to @mention) |
| `gojira assign <KEY> <email>` | Assign by email (`--json`; resolves to accountId) |
| `gojira update <KEY>` | Update `--summary`, `--description`, `--priority`, `--labels`, `--due-date`, `--components`, `--fix-versions`, or paired `--story-points VALUE --story-points-field customfield_N` (`--json`; explicit empty values clear supported fields) |
| `gojira config` | Manage configuration: `init`, `set`, `get`, `path`, `test` |
| `gojira version` | Version marker + build revision (`--json`) |

Lifecycle commands use only transitions currently available on the issue, never workflow names.
Missing destination data is not a candidate; zero or multiple candidates are listed in a
`validation_error` without writing. Use `move` to choose a transition manually.

**Delete is permanent.** It does not cascade to subtasks (Jira may reject a parent),
does not re-fetch the deleted issue, and is never retried automatically. A 5xx or
transport failure has `mutationState: unknown`: verify before retrying. Permission
errors use the existing classified error envelope. Lifecycle validation used local
HTTP simulations only; no live Jira verification or deletion was performed.
TUI lifecycle shortcuts and typed-key deletion confirmation are not implemented yet;
the existing `t` transition picker remains available.

### Agent usage examples

```bash
# Feed a sprint to an AI agent
gojira export --board 1 | jq '.[] | select(.assignee == "René")'

# What can this ticket become? (transition picker as JSON)
gojira move ARA-1892 --json

# Close/reopen only when exactly one destination-category candidate is available
gojira close ARA-1892 --json
gojira reopen ARA-1892 --json

# PERMANENT deletion: confirmation must match the issue key exactly
gojira delete ARA-1892 --confirm ARA-1892 --json

# Full requirement context as JSON
gojira get ARA-1892 --json

# Create a ticket from a markdown requirement file
gojira create --project ARA --type Task --summary "Add export" --description-file req.md

# Create a sub-task beneath an existing issue (defaults to --type Sub-task)
gojira create --project ARA --parent ARA-1892 --summary "Verify export" --json

# Create a ticket straight into the active sprint (skip the backlog)
gojira create --project ARA --type Task --summary "Sprint work" --board 1

# List candidate sprints, then move exactly one issue
gojira sprint list --board 1 --state active,future --json
gojira sprint move ARA-1892 --to 42 --json

# Move an issue from future and active sprints to the backlog
gojira backlog ARA-1892 --json

# Comment and notify the PM
gojira comment ARA-1892 "Ready for review" --mention pm@devoost.com

# Replace a description using plain text converted to Jira ADF
gojira update ARA-1892 --description $'First paragraph\n- checklist item'

# Replace due date, components, and fix versions after Jira editability checks
gojira update ARA-1892 --due-date 2026-10-31 --components "API,Web" --fix-versions "v1.0,v1.1"

# Clear these fields explicitly (omitting a flag leaves its field unchanged)
gojira update ARA-1892 --due-date= --components= --fix-versions=

# Set or explicitly clear the site's selected numeric story-points field
gojira update ARA-1892 --story-points 5.5 --story-points-field customfield_10016
gojira update ARA-1892 --story-points= --story-points-field customfield_10016

# Include the same selected field in human or JSON reads
gojira get ARA-1892 --story-points-field customfield_10016 --json
```

`create --parent KEY` accepts a canonical Jira issue key and requires type
`Sub-task` when `--type` is explicit. It cannot be combined with `--board`:
child sprint membership is not established by this command. Without `--parent`,
the default remains `Task`. GoJira validates the flag combination locally but
Jira decides whether the parent exists and permits child creation.

`backlog KEY` sends one request to Jira's [Cloud backlog endpoint](https://developer.atlassian.com/cloud/jira/software/rest/api-group-backlog/),
without first reading sprint membership. The documented operation removes future
and active sprint membership; preservation of closed sprint history has not
been independently proven. A 204 response confirms the request succeeded, not
independently verified backlog membership. No live Jira verification has been
performed.

Sprint movement sends exactly one write, never a synthetic multi-write loop to
rewrite closed-sprint history. Board and sprint IDs must be positive integers.
Human output confirms Jira accepted the request, not independently verified
membership. Failed or ambiguous writes are never automatically retried; HTTP 500
and transport drops retain mutation state `unknown`. No live Jira verification
was performed for sprint selection or movement (local HTTP simulations only).

JSON modes write one valid JSON document to stdout, with diagnostics on stderr.
### Search completeness

Search defaults to 50 issues; `--limit <= 0` also means 50. Jira caps each
page at 100, so larger limits follow `nextPageToken` across pages.
`--all` fetches up to a safety cap of 10,000 issues and cannot be combined
with an explicit `--limit`. Pagination fails on repeated tokens or after
1,000 pages rather than silently claiming completeness.

Default `--json` stdout remains the bare TicketJSON v1 array. A truncated
result exits successfully and writes one warning object to stderr:

```json
{"schemaVersion":"v1","code":"results_truncated","returned":50,"estimatedTotal":696,"hint":"Rerun with --all or a larger --limit, or narrow the JQL; do not report this list as complete."}
```

`estimatedTotal` is optional: truncated searches make one best-effort
approximate-count request with the same JQL. This is an **estimate**, requires
bounded JQL, and is omitted if counting fails. Counting with `ORDER BY` has
not been verified. Human output uses a plain `Warning:` on stderr instead.
Complete results emit no warning.

`--meta` implies JSON and emits
`{"schemaVersion":"v1","issues":[],"returned":0,"truncated":false}`
on stdout, with optional `estimatedTotal` and no stderr warning. Use this
metadata to check completeness, including when `--all` reaches its cap.
Search uses `/rest/api/3/search/jql`; the old `/rest/api/3/search`
GET/POST endpoints are marked by Atlassian as currently being removed.

The curated `TicketJSON` schema is used by `get --json`, `search --json`,
`export`, TUI ticket/column copy, and successful JSON mutations. `get --json`,
ticket copy, and `create`/`update`/`assign`/`move --to`/`backlog`/`comment`/`log` with
`--json` produce one object; mutation commands re-fetch the full issue after the
write. Search, export, and column copy produce a top-level array. Each ticket
object carries `schemaVersion: "v1"`. Human mutation output is unchanged when
`--json` is omitted.

TUI ticket detail shows readable created/updated dates, supplied logged time,
embedded worklog entries (with a truncation count when a total is supplied), and
existing comments in Jira order, including an explicit empty state. The existing
scrolling and navigation keys are unchanged. TUI comment entry is not implemented
in this slice.

Full issue reads request optional `timetracking` and `worklog` fields; board,
search and export field sets are unchanged. The embedded `fields.worklog` shape
and `fields.timetracking.timeSpent` were **not confirmed** in the official
Atlassian v3 worklog/comment pages consulted: only supplied values are exposed,
with no inferred total or pagination requests. Those pages document ADF worklog
comments; `log --show --json` continues to emit flattened string comments and
also accepts legacy strings or missing comments. No live Jira verification was
performed (local tests only).

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
| `fixVersions` | array of strings | Omitted when empty; fix-version display names. |
| `dueDate` | string or null | Always; Jira calendar date in `YYYY-MM-DD` form, or `null` when unset or unavailable. |
| `storyPoints` | number or null | Present only when a validated `--story-points-field customfield_N` selector is supplied to `get`, or when `update --json` writes that selected field; the exact Jira number or `null`. |
| `description` | string | Always; flattened ADF text, or empty string when unavailable. |
| `created` | string | Always; RFC3339 when recognized, empty when unavailable, otherwise the original Jira value. |
| `updated` | string | Always; RFC3339 when recognized, empty when unavailable, otherwise the original Jira value. |
| `timeLogged` | string | Optional `fields.timetracking.timeSpent` from full issue reads (`get` and mutation re-fetch); omitted when absent or empty, never calculated from entries. |
| `worklogs` | array of objects | Omitted when empty; embedded entries in Jira order with string `author`, `timeSpent`, RFC3339-normalized `started` (original on parse failure), and flattened string `comment`. No additional reads. |
| `worklogsTotal` | integer | Only included when Jira supplies `fields.worklog.total`, including zero. Entries may be truncated; absence is not zero. |
| `comments` | array of objects | Omitted when empty; comments in Jira order. Each object always has string `author`, flattened string `body`, and string `created` with the same date rules. |
| `parent` | object | Omitted when absent or null; `get` parent with string `key`, `summary`, `status`, and optional `statusCategory` key as supplied by Jira. |
| `sprint` | object | Omitted when absent/null; integer `id`, string `name` and `state` supplied by Agile Jira. |
| `subtasks` | array of objects | `get`, board/export/TUI children in Jira order, with the same nested fields as `parent`; omitted when absent/null, but an explicit empty list is `[]`. No per-child reads. |
| `subtaskProgress` | object | Available only when Jira supplies a non-null `subtasks` list and every child has a key and recognized status category (`new`, `indeterminate`, `done`); integer `done` and `total`, including `0/0` for `[]`. Done uses the category key case-insensitively, never the status name. Unknown child categories retain the children but omit progress. |

Read surfaces can only populate fields requested and returned by their Jira
endpoint. In particular, board-backed export and TUI column copy request basic
issue fields, so description, comments, and timestamps may be empty or omitted.
Sparse Jira responses preserve the required empty-string fields and the
`"Unassigned"` default described above. `dueDate` is always present as either a
`YYYY-MM-DD` string or `null`. `storyPoints` is omitted unless a field selector
was supplied; with a selector it is always a JSON number or `null`. A selected
field missing from Jira's response, or returned as another JSON type, is a read
error rather than an omitted property. `get` keeps its full issue GET and adds
exactly one best-effort Agile issue GET (`fields=sprint`, no retry); failure or
missing sprint evidence omits `sprint` without failing the command. Human output
shows `Sprint: name (state)` only when known. Board/export/TUI reads request
`subtasks` and `sprint` in bulk, without per-card reads; search is unchanged.
Cards show verified `done/total` progress and their own sprint name only when it
differs from the board's active sprint. Missing progress is not zero.
Availability of nested child status categories depends on Jira's response.
Synthetic tests cover this shape;
live Jira Cloud compatibility has not been verified.

`boards --json` and `projects --json` expose their own list shapes.
`move --json` without `--to` retains its transition array, and
`log --json --show` retains its worklog array; both print `[]` when Jira returns
no entries, never `null`. `version --json` has its own `version`/`revision`/`dirty`
schema. `config` and `tui` are intentionally not JSON commands: `config` manages
local settings for humans and `tui` is interactive.

For `get`, `search`, `boards`, `projects`, `move`, `log --show`, and `version` with `--json`, and for `export`
(which is always JSON), configuration failures report `configuration_error`,
Jira read failures report `read_failed`, and result encoding/writer failures
report `output_failed`. Export with no boards reports `validation_error`.
Each exits nonzero and writes one v1 error object to stderr with
`mutationState: "not_applied"`; `get` includes the issue key. Automatic export
board/sprint selection diagnostics remain on stderr. No success JSON is written
before result encoding begins. An output writer can fail after a partial stdout
write, so output is not guaranteed atomic. Invalid arguments retain the root
command's `validation_error` behavior.

For `update`, `--due-date` is validated as a real `YYYY-MM-DD` calendar date,
and comma-separated component/version lists reject empty or duplicate names
before any Jira request. When any of those three flags is present, GoJira first
loads that issue's edit metadata and refuses the write if a requested field is
not editable. Component and fix-version names are checked locally when Jira
supplies `allowedValues`; otherwise Jira's write rejection remains visible.
The metadata read never causes a write retry, and the PUT is still dispatched
at most once.

Story points use no field-name heuristics. `--story-points` and
`--story-points-field customfield_N` must be supplied together; malformed IDs,
non-decimal values, and non-finite values are rejected before any Jira request.
An explicitly empty `--story-points=` sends JSON `null`. Before the PUT, edit
metadata must contain the selected field with `schema.type: "number"` and a
`set` operation. Selector-aware reads request the normal full fields plus only
the selected custom field, and preserve decimal precision in JSON output.

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

### Jira access errors for agents

CLI JSON failures remain one v1 error envelope on stderr, with empty stdout
and exit 1. HTTP 401 uses `authentication_failed`, 403 `permission_denied`,
and 404 `not_found_or_no_permission` (Jira can hide inaccessible resources).
These envelopes add `httpStatus` and an actionable `hint`; `mutationState`
still distinguishes `not_applied`, `applied`, and `unknown`. Never repeat an
applied write, or retry an unknown write without verification. A
`partial_failure` retains its code and adds the classified `cause` instead.
Other failures retain their existing codes and fields. Human output appends
the same hint after the existing error line on stderr.

Failed Jira mutations in CLI JSON and opt-in MCP write tools additionally expose
`httpStatus`, and optional `errors` (field-to-message object) and `errorMessages`
(string array). For example, a rejected create may include
`"httpStatus": 400, "errors": {"summary": "Summary is required"}` without changing
`code`, `message`, or `mutationState`. Only these structured Jira error fields
are parsed, from at most 64 KiB; oversized, malformed, non-JSON, or unreadable
error bodies yield only the HTTP status. Each collection has at most eight
entries; keys and messages are limited to 256 UTF-8 bytes after redaction.
Configured email, token, and Basic-auth encoded credentials are replaced in
both keys and messages before truncation. Arbitrary unrelated secrets are not
recognized: treat Jira messages as untrusted data. Full raw response bodies and
unfiltered mutation transport errors are never exposed. Human CLI errors use
the same sanitized details. Transport failures remain generic with an unknown
outcome and no HTTP status. Writes are never automatically retried; verify an
unknown outcome in Jira before deciding whether to retry.

`get` still succeeds when its optional Agile sprint read fails. Its JSON
adds `warnings: [{code, message, httpStatus?}]` with code `sprint_unavailable`;
classified causes appear in `message`. Warnings are omitted when empty, and
no sprint is fabricated. Human mode sends warnings to stderr. Access errors
do not trigger permission pre-checks or automatic 4xx retries.

### CLI and TUI parity

The matrix below compares only the CLI and the TUI. The full three-surface matrix,
including the MCP server and per-option gaps, is in
[docs/interface-parity.md](docs/interface-parity.md).

| Capability | CLI | TUI |
|---|---|---|
| Read ticket detail (dates, time, comments, worklogs) | `get` (`--json`) | `Enter` (detail modal) |
| Search | `search` (`--json`) | `s` and `/` |
| Create ticket (optionally in a sprint) | `create` | `n` |
| Edit fields | `update` | `e` (description), `U` (summary, priority, labels), `E` (other fields) |
| Add comment | `comment` | `m` |
| Log work / show worklog | `log` (`--show`) | `w` |
| Transition by name | `move` | `t` |
| Close / reopen by status category | `close`, `reopen` | `X` |
| Delete permanently (typed confirmation) | `delete --confirm KEY` | `D` |
| List sprints | `sprint list` | `S` chooser |
| Move to sprint | `sprint move` | `S` |
| Move to backlog | `backlog` | `b` |
| Assign (by email) | `assign` | `A` |
| Child progress and sprint | `get --json` | board cards |

Agent-oriented extras exist only in the CLI: `--json` on every command except
`config` and `tui`, `export`, and the stable error envelope. Mutations never
retry automatically; a failed write reports `not_applied`, `applied`, or
`unknown`. None of the commands above has been verified against a live Jira, so
validate on a disposable issue first; deletion is permanent.

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
| `A` (Shift+A) | Assign the selected card by email: type the assignee's email, `Enter` assigns once, `Esc` cancels |
| `S` | From a selected card or ticket detail, choose a future/active sprint of the loaded board; Enter moves once, Esc cancels |
| `b` | From a selected card or ticket detail, move to backlog with one request |
| `X` | Close a selected card/detail, or reopen a Done-category issue; exactly one destination-category transition is required, otherwise use `t` |
| `D` | Permanently delete a selected card/detail: type its exact key and press Enter; Esc cancels before submission. One DELETE, never automatically retried |
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
| `m` | Add a comment in ticket detail; `Enter` adds a line, `Ctrl+S` submits once, `Esc` cancels |
| `E` (Shift+E) | Edit due date, components, fix versions, and story points in one bounded form |
| `w` | Add worklog (cancel or success returns to this detail modal) |
| `A` (Shift+A) | Assign this ticket by email (same form as on the board) |
| `U` (Shift+U) | Edit summary, priority and labels in one bounded form: `Tab`/`Shift+Tab` move, `Ctrl+S` saves, `Esc` cancels |
| `o` | Open in browser |
| `Esc` | Back to the board or search results |

The `U` form shows the current summary, priority name and comma-separated labels. Only what
you changed is sent, with the same update `gojira update --summary --priority --labels`
builds (priority by name, as in the CLI). Summary and priority cannot be emptied and labels
cannot be cleared, as in the CLI and the MCP tool; label names are trimmed and none may be
empty. Saving with no change sends nothing. One `Ctrl+S` sends one update and one refresh,
never retried automatically: after a failure the text stays for another `Ctrl+S`, and if
the update was applied but the refresh failed, `Ctrl+S` retries only the refresh. `Esc`
leaves the form but cannot cancel a request already sent, and a late reply is discarded
if you moved to another ticket or board. On success the open ticket and its board card or
search row show the new values at once.

Assignment resolves the typed email to a Jira account with the same lookup as
`gojira assign` (an exact email match, never a guess); an unknown or ambiguous
email assigns nothing and shows the error. One `Enter` sends one lookup and one
assignment, never retried automatically, and repeated keys cannot start a second
request. `Esc` leaves the form but cannot cancel an assignment already sent to
Jira, and a late reply is discarded if you moved to another ticket or board. On
success the board (or the search you came from) refreshes.

Comment entry converts plain text to ADF (mentions are not supported) and rejects
empty comments locally. After a successful write, a failed detail refresh offers
only a refresh retry, never a second comment POST. Write failures show the
classified error and `not_applied` or `unknown` mutation state; writes are never
automatically retried. No live Jira verification was done.

The description editor accepts multiple lines and converts an actual plain-text
edit to Jira ADF. Replacing a description this way may discard rich ADF formatting
that plain text cannot represent; confirming unchanged text performs no write. A
successful update refreshes the open ticket detail. If the update succeeds but
refresh fails, the editor marks the update as already applied and `Ctrl+S` retries
only the detail refresh; closing that state does not roll back the Jira update.
Other failures keep the editor text and ticket context available for retry or
cancel.

In the separate `E` field editor, `Tab`/`Shift+Tab` switch fields, `Ctrl+D`
clears the focused field, `Ctrl+S` saves, and `Esc` cancels. Dates require a
real `YYYY-MM-DD` date; components and fix versions are comma-separated exact
Jira names without duplicates; story points require a finite decimal and a
canonical `customfield_N` ID. Clearing date/points sends JSON null; clearing a
list sends an empty array. Unchanged fields are omitted, and a no-op sends no
PUT. The editor checks this issue's edit metadata and allowed list values before
one combined update. A Scrum board with field-based estimation supplies the
validated story-points field ID; otherwise enter the ID manually (the displayed
field name is never used as an ID). Leaving a manually entered ID with `Tab`
(or pressing `Ctrl+S`) first loads the actual selected value from Jira; saving
is disabled while it loads. Changing the ID discards the previously selected
points value. If the selected GET fails or returns missing/invalid data, no
write is allowed: retry the GET with `Ctrl+S` or clear/change the ID. An invalid
nonempty ID blocks all updates, including edits to unrelated fields. After a
successful PUT, a failed detail refresh leaves an applied state:
`Ctrl+S` retries only the GET, never the PUT. An uncertain write failure retains
the form; check Jira before retrying because the mutation may already have landed.
An HTTP request already dispatched cannot be undone by exiting the TUI.

## Project context

GoJira can remember which Jira project you work on, without creating any file in
your repository. The effective project is resolved in this order:

1. the `--project` flag (where a command supports it),
2. the `GOJIRA_PROJECT` environment variable,
3. `git config gojira.project`, read through Git itself,
4. the user-level `default_project` (`gojira config set project KEY`).

Empty layers fall through; a value that is not a Jira project key (for example
`demo`) fails with an error naming the layer instead of silently falling back.
Outside a Git repository the Git layer is ignored.

```sh
gojira init                          # pick a project from a list (terminal)
gojira init --project ARA --yes      # no terminal: explicit key, e.g. an agent
gojira context --json                # resolved project, its source and who you are
```

`gojira init` writes `[gojira] project = KEY` to the local, untracked
`.git/config` of the current clone with `git config --local`. It never edits
tracked files and never stores credentials. With a terminal it shows a project
list: arrows (or `k`/`j`) move, typing filters by key or name, Enter selects,
Esc or Ctrl+C cancels without writing. Without a terminal it needs
`--project KEY --yes`. The key is checked against Jira before it is written.
If a higher-priority layer still wins (for example `GOJIRA_PROJECT`, or a
global or worktree Git value), `init` says so instead of implying the new value
is in force.

`gojira context --json` prints `schemaVersion`, `project` (`key`, `source`, and
the Git `scope` when applicable) or `null`, `needsProject`, and `identity`
(`accountId`, `displayName`, `active`; never the email). With no project it exits
0 with `project: null` and `needsProject: true` so an agent asks the human
instead of guessing. Commands such as `create`, `search` and `backlog` are not
yet wired to this resolution; keep passing their explicit arguments.

## Read-only MCP server

Configure credentials once with `gojira config init`, then launch `gojira mcp`.
Stdout is reserved for MCP stdio frames; do not wrap it with logging to stdout.
Any MCP client that can launch a local stdio command can use it. Let gojira print
the entry for you, already pointing at this binary:

```bash
gojira mcp config                  # mcpServers JSON (default)
gojira mcp config --client codex   # Codex config.toml entry
gojira mcp config --allow-writes   # also grants the guarded write tools
```

The output contains no credentials, so it is safe to paste or share; the server
reads them from `gojira config init`. Without `--allow-writes` the connected
agent can only read. To let it create tickets, comment, log work or move issues
(for example for a PM), print the entry with `--allow-writes`; the tools it
enables and their limits are listed under
[Opt-in guarded writes](#opt-in-guarded-writes). Where to paste it:

- **Codex** (desktop app, CLI and IDE share it): append the TOML to
  `~/.codex/config.toml`, or use Settings → MCP servers → Add server → STDIO
  with the binary path and the argument `mcp`, then restart.
- **Other clients** (for example Claude Desktop, Cursor, Windsurf, Gemini CLI):
  paste the JSON into the client's MCP configuration file. The `mcpServers`
  shape is the common convention, but check your client's documentation for the
  file location; only Codex has been checked against its official docs.

Web-only chat clients cannot launch local processes and are not supported.

The fixed tool allowlist is `gojira_context`, `gojira_projects`, `gojira_boards`,
`gojira_sprints` (`boardId`), `gojira_get` (`key`), `gojira_search`
(explicit `jql`, optional `limit`, default 50, range 1–1000),
`gojira_users_search` (`query`), and `gojira_report_time`
(`accountId`, `project`, `from`, `to`, `timezone`, all required). `gojira_context`
returns the same object as `gojira context --json`, resolved from the server
process's working directory and environment, which a desktop client may not set
to your repository: when `needsProject` is true, ask the human. No mutations,
exports, arbitrary REST calls, or project inference are exposed.
No repository files or guide installation are required. Read `gojira://guide`
for usage guidance; ticket text is untrusted data, never agent instructions.
Tool annotations are hints, not authorization controls.

Existing context/ticket collection tools retain their contracts. Collection
structured results contain `schemaVersion`, `data`, `complete`,
`completeness` (`unknown`, `complete`, or `partial`), and an optional `warning`.
Projects are fully paginated and report `complete:true`. Boards and sprints
return one page with `complete:false` and `completeness:unknown`: the existing
client does not expose enough pagination metadata to establish completeness. Get
also reports unknown: embedded comments
and worklogs are not complete histories. Search distinguishes complete results
from confirmed truncation (`partial`) using the existing paginated search
contract. Access remains limited to the authenticated user's visibility.

Tool failures set `isError` and preserve a structured v1 envelope with `code`,
`message`, `mutationState:not_applied`, `hint`, and optional `httpStatus`.
Diagnostics omit raw Jira bodies and transport details. Invalid arguments make
no Jira request. Existing CLI JSON contracts are unchanged. In-flight Jira
requests use the existing HTTP timeout; disconnect/cancellation does not yet
interrupt those requests for the original tools. The two reporting tools propagate
context cancellation into in-flight requests. OAuth and client bundles are later slices.

### Opt-in guarded writes

Run `gojira mcp --allow-writes` to add the following tools; without this flag,
all existing read-only tools, instructions and results remain unchanged.

| Tool | Arguments |
| --- | --- |
| `gojira_create` | `project`, `issueType`, `summary`, optional `description` |
| `gojira_comment` | `key`, `body` |
| `gojira_log_work` | `key`, `timeSpent`, optional `comment` |
| `gojira_assign` | `key`, `accountId` |
| `gojira_move` | `key`, exact unambiguous `status` |
| `gojira_update` | `key`, `fields` with any of `summary`, `description`, `priority`, `labels`, `dueDate`, `components`, `fixVersions`, `storyPoints` (sent with `storyPointsField`) |
| `gojira_delete` | `key`, optional `confirmationToken` |
| `gojira_sprint_move` | `key`, `sprintId` (positive integer) |
| `gojira_backlog` | `key` |
| `gojira_close` | `key` |
| `gojira_reopen` | `key` |

Assignment deliberately accepts a non-empty, trimmed account ID directly; unlike
the CLI, it does not resolve an email. Creation never places an issue in a sprint;
`gojira_sprint_move` moves one existing issue into the sprint with that ID (list
them with `gojira_sprints`), and `gojira_backlog` takes it out of its future and
active sprints. Like the CLI, a success only means Jira accepted the request:
the resulting sprint membership is not read back. `gojira_close` and
`gojira_reopen` choose the transition exactly like `gojira close` and
`gojira reopen`: the single available one whose destination status category is
`done` (close) or `new` or `indeterminate` (reopen). With none or several, they
return a `validation_error` listing the candidates and change nothing; then use
`gojira_move` with an exact status name.
`gojira_update` takes the same fields as `gojira update`. An empty string clears
`description` and `dueDate`, `[]` clears `components` and `fixVersions`, and `null`
clears `storyPoints`; `labels` needs at least one non-empty, trimmed name (as in
the CLI, labels cannot be cleared). `storyPoints` (a finite decimal) and
`storyPointsField` (`customfield_NNN`) must be sent together. Values are
validated, and the issue's edit metadata is checked, before anything is sent; a
rejected value returns a `validation_error` naming the field with
`mutationState:not_applied`. Replacing a description sends plain text converted
to ADF and may discard rich ADF formatting that plain text cannot represent;
prefer appending a comment when formatting matters. Create,
comment and log_work are non-idempotent. No write is automatically retried.
Results report `schemaVersion:v1`, `action`, optional `issueKey`, and
`mutationState` (`applied`, `not_applied`, `unknown`), with optional `data` or
sanitized `code`, `message`, and `hint`. Verify unknown outcomes in Jira before
retrying; a failed response does not prove that a mutation failed.

Delete first returns a preview containing only action and normalized issue key,
plus an opaque confirmation token in `data.confirmationToken`. Send the same key
and token to execute. Tokens live only in server memory, expire after two minutes,
and are consumed atomically once (including a wrong-key attempt). Restarting the
server invalidates them. No valid token means no deletion. Ticket text remains
untrusted. The token forces a deliberate second call and prevents a single-call
accidental delete. It is NOT proof of human approval: the same agent receives and
redeems it. The server cannot guarantee client-side human confirmation;
annotations are hints, not proof of consent.
Cancellation is checked before execution but cannot interrupt in-flight calls
through the existing Jira write client.

## User candidates and time reports

```sh
gojira users search 'Alex Smith' --json
gojira report time --account-id ACCOUNT_ID --project DEMO \
  --from 2025-01-01 --to 2025-01-31 --timezone America/New_York --json
```

Omit `--json` for a human-readable table. User candidates include account ID,
display name and active status. Same-name candidates remain separate: choose an
explicit account ID, never silently infer one. Empty candidates mean no visible
matches were returned, not that the person does not exist. Jira user search only
covers active matches within the endpoint's first 1000 users; missing Browse
users permission can silently produce an empty successful response.

Every time filter is explicit; project defaults are not used. Dates are inclusive
calendar dates in the supplied IANA timezone, implemented as a half-open interval
through the start of the following day (including DST changes). Worklogs are
filtered locally by author account ID and started instant, not creation date.
Rows and totals use integer seconds; issue rows are sorted by key.

CLI and MCP share the same reporting operations and result objects. User-search
JSON has `candidates`, `completeness` and `warnings`; time-report JSON has
`filters`, `rows`, `totalSeconds`, `complete`, `completenessScope`, scan counters
and `warnings`. Both include `schemaVersion`. MCP text matches structured JSON.
Human reports display **PARTIAL** whenever totals are incomplete and show warnings.
JSON warnings stay inside the single stdout result; failure diagnostics go to stderr.

Reports scan individually paginated issue worklogs, bounded to **1000 issues** and
**2000 page requests**. Caps, malformed paging or interrupted scans can yield
partial totals: inspect `complete` and `warnings` before using the total. Even
`complete:true` means only **visible to the authenticated user** within the scan,
not organization-wide coverage or a transactional snapshot. Project, issue-security
and worklog visibility permissions may hide data. No PDF generation, export,
mutation or report persistence is provided.

## Optional agent guide

Nothing is installed unless you explicitly run `gojira guide install --yes`.
Printing or previewing the guide does not create files or directories.

```sh
gojira guide                                      # print Markdown to stdout
gojira guide install --path docs/gojira-guide.md --exclude        # preview only
gojira guide install --path docs/gojira-guide.md --exclude --yes  # install
```

The preview shows the destination, content size in bytes, and the exclusion line
when `--exclude` is selected. Installation creates parent directories (`0755`)
and a new guide file (`0644`). Existing targets are refused without modification;
there is no overwrite flag. Empty paths, directories, and filenames `AGENTS.md`
or `CLAUDE.md` (case-insensitive) are always refused.

Choose a per-user location with `--path`, or use `--exclude` for a repository-local
file. Exclusion requires the destination to be inside a Git working tree and adds
its repository-relative path once to the local exclude file resolved by Git
(including linked worktrees), preserving existing lines. Exclusion errors stop
installation before the guide is written. The exclusion is an anchored, literal
pattern, so it matches only that file. To keep the guide from being staged by
accident, a target inside a Git working tree is refused unless `--exclude` is
passed (the preview refuses it too); if Git cannot be run, the state is unknown and
the installation is refused. After applying `--exclude` the installer asks Git whether the
guide is effectively ignored; Git gives the local exclude file the lowest precedence, so
a negation in the repository ignore rules (for example `!guide.md`) overrides it, and in
that case the installation is refused and the guide is not written. The exclusion line
already added stays in the local exclude file; it is harmless and can be removed by hand.
A path outside any repository needs no flag. No Jira
connection is needed.
For a file-free alternative, read the MCP resource `gojira://guide`.

## Project structure

```
gojira/
├── main.go                    # Entry point
├── cmd/                       # Cobra CLI: thin adapters over internal/*
│   ├── root.go                # Root command + credential resolution
│   ├── tui.go                 # TUI launcher + first-run wizard
│   ├── boards.go projects.go get.go search.go export.go
│   ├── move.go close.go delete.go sprint.go backlog.go
│   ├── log.go create.go comment.go assign.go update.go
│   ├── context.go init.go     # Project context: resolve, show, store
│   ├── mcp.go                 # Wiring of the MCP server (stdio)
│   ├── json.go                # Shared JSON/error envelope helpers
│   └── config.go              # config init/set/get/path/test
├── internal/
│   ├── jira/                  # Jira domain: one file per area, one Client type
│   │   ├── client.go          # Client, HTTP transport, retries, errors
│   │   ├── client_issues.go   # Issue reads and writes, transitions
│   │   ├── client_boards.go   # Boards, sprints, backlog
│   │   ├── client_search.go   # JQL and text search with pagination
│   │   ├── client_projects.go # Paginated projects
│   │   ├── client_users.go    # Identity and account lookup
│   │   ├── client_worklog.go  # Worklogs and comments
│   │   ├── paths.go           # ALL API endpoints — single source of truth
│   │   └── types.go           # Types + ADF parser + curated TicketJSON
│   ├── projectctx/            # Project resolution policy and context use case
│   │   ├── projectctx.go      # flag > env > git > user default; ReadContext
│   │   └── git.go             # Git adapter behind the GitSource interface
│   ├── mcpserver/             # Read-only MCP adapter: one table entry per tool
│   │   ├── server.go          # Server wiring, dependency injection
│   │   ├── tools.go           # Tool table: schema, validation, handler
│   │   └── schema.go
│   ├── picker/                # Standalone terminal project selector
│   ├── tui/                   # Bubble Tea TUI (single model, no sub-model routing)
│   │   ├── tui.go             # Model: boards, kanban, detail, transition, worklog, create, sprint
│   │   ├── setup.go           # First-run configuration wizard
│   │   └── tui_test.go        # Render geometry tests
│   └── config/                # XDG config + keychain token storage only
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
network access must be prohibited.

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
