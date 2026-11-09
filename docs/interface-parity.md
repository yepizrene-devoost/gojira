# CLI, MCP and TUI capability parity

GoJira exposes the same Jira business operations through three surfaces: the
CLI (humans and scripts), the MCP server (agents) and the TUI (humans, visually).
The goal ([issue #30](https://github.com/yepizrene-devoost/gojira/issues/30),
completed in [issue #33](https://github.com/yepizrene-devoost/gojira/issues/33)) is
that anything you can do in one surface you can also do in the others, with
equivalent options. Controls differ on purpose: flags in the CLI, structured
arguments in MCP, selectors and forms in the TUI.

This matrix is the checklist for that goal. Every row below is implemented on all
three surfaces; the limits of a row are written in the row itself, and the one
capability left out on purpose is listed under "Different on purpose". A change
that opens a gap must mark its row *Partial* or *No* again.

| Status | Meaning |
| --- | --- |
| Yes | The capability exists with the options listed |
| Partial | The capability exists but an option is missing (see the note) |
| No | The capability does not exist on that surface |
| In progress | A change for issue #30 is underway and not yet delivered |

## Read and search

| Capability | CLI | MCP | TUI |
| --- | --- | --- | --- |
| Ticket detail | `get` | `gojira_get` (embedded history may be partial) | `Enter` detail |
| Search by JQL with a limit and truncation signal | `search` | `gojira_search` | `s`, then `Tab` for JQL; `Ctrl+L` selects 1–10,000 or `all` (10,000 cap), with truncation signal (`/` remains text search) |
| List projects | `projects` | `gojira_projects` | Intentionally excluded |
| List boards and sprints | `boards`, `sprint list` | `gojira_boards`, `gojira_sprints` | Boards list, sprint chooser |
| Export tickets | `export` | `gojira_export` (completeness unknown) | `E`: current or explicit board ID, first active sprint or board fallback, optional sprint ID or all-board override; clipboard only. Sprint discovery and export completeness are unknown (2,000-issue export safety threshold). `C` still copies a column. |
| Project context and identity | `context` | `gojira_context` | Uses the board's project |

## Change tickets

| Capability | CLI | MCP | TUI |
| --- | --- | --- | --- |
| Create (type, summary, description) | `create` | `gojira_create` | `n` |
| Create under a parent | `create --parent` | `gojira_create` with `parent` and explicit `issueType: Sub-task` | `n` with optional parent key |
| Create inside an active sprint | `create --board` | `gojira_create` with positive `boardId` | `n` (active sprint) |
| Edit summary, priority, labels | `update` | `gojira_update` | `U` |
| Edit description | `update` | `gojira_update` (plain text replaces it) | `e` |
| Edit due date, components, fix versions, story points | `update` | `gojira_update` | `E` |
| Comment | `comment` | `gojira_comment` | `m` |
| List comments (complete, paginated) | `comment --list` (`--json` with `total`) | `gojira_comments` (read) | `C` |
| Read one comment by id | `comment --id <ID>` (`--json`) | `gojira_comments` (ids in the listing) | `C` |
| Edit comment body (plain text as ADF; whole body replaced) | `comment --id <ID> --text "..."` | `gojira_update_comment` | `C`, then `e` |
| Delete comment (destructive, confirmed) | `comment --id <ID> --delete --confirm <ID>` | `gojira_delete_comment` (preview plus single-use token) | `C`, then `d` + `y` |
| Attach a file | `attach` (several files) | `gojira_attach` (one path per call, any readable path) | `f` (one path) |
| Log work | `log` | `gojira_log_work` | `w` |
| Assign | `assign` (by email) | `gojira_assign` (by email or account ID) | `A` (by email) |
| Transition by status name | `move` | `gojira_move` | `t` |
| Close or reopen by status category | `close`, `reopen` | `gojira_close`, `gojira_reopen` | `X` |
| Delete | `delete --confirm KEY` | `gojira_delete` (preview plus single-use token) | `D` (type the key) |
| Move to a sprint | `sprint move` | `gojira_sprint_move` (by sprint ID) | `S`, `a` |
| Move to the backlog | `backlog` | `gojira_backlog` | `b` |

MCP write tools exist only when the server starts with `--allow-writes`.

## Reports and people

| Capability | CLI | MCP | TUI |
| --- | --- | --- | --- |
| Search users | `users search` | `gojira_users_search` | `u` from boards or Kanban; identity, completeness and permission warnings |
| Time report by person and date range | `report time` | `gojira_report_time` | `r` from boards; explicit filters, completeness and warnings |

## Different on purpose

These are not gaps: they serve the surface they live on.

- Project listing is intentionally excluded from the TUI: the board list is its
  entry point, and a project with no visible board is reachable through the CLI
  or MCP.
- Setup and credentials (`config`, `init`, `mcp config`, `guide`) are CLI-only;
  the MCP server and the TUI read the configuration the CLI wrote.
- `--json` output and the stable error envelope belong to the CLI; MCP returns
  its own structured results with the same mutation-state semantics
  (`applied`, `not_applied`, `unknown`).
- Delete confirmation differs by surface (typed key, `--confirm`, single-use
  token). The MCP token forces a second call but is **not** proof of human
  approval.
- Keyboard navigation, overlays and toasts exist only in the TUI.

## Keeping it current

A change to any operation or option on one surface updates this file in the same
work unit, and either lands on the other two surfaces or leaves the row marked
*Partial* or *No* with the gap visible. Mutations are never retried
automatically on any surface.
