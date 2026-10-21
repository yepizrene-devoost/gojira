# CLI, MCP and TUI capability parity

GoJira exposes the same Jira business operations through three surfaces: the
CLI (humans and scripts), the MCP server (agents) and the TUI (humans, visually).
The goal ([issue #30](https://github.com/yepizrene-devoost/gojira/issues/30)) is
that anything you can do in one surface you can also do in the others, with
equivalent options. Controls differ on purpose: flags in the CLI, structured
arguments in MCP, selectors and forms in the TUI.

This matrix is the checklist for that goal. It is **not** a claim of full parity:
every row that says *Partial*, *No* or *In progress* is a known gap.

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
| Search by JQL with a limit and truncation signal | `search` | `gojira_search` | **Partial**: text search only (`s`, `/`), no JQL |
| List projects | `projects` | `gojira_projects` | No |
| List boards and sprints | `boards`, `sprint list` | `gojira_boards`, `gojira_sprints` | Boards list, sprint chooser |
| Export tickets | `export` | No | **Partial**: copy as JSON (`C`) |
| Project context and identity | `context` | `gojira_context` | Uses the board's project |

## Change tickets

| Capability | CLI | MCP | TUI |
| --- | --- | --- | --- |
| Create (type, summary, description) | `create` | `gojira_create` | `n` |
| Create under a parent | `create --parent` | No | No |
| Create inside a sprint | `create --board` | No | `n` (active sprint) |
| Edit summary, priority, labels | `update` | `gojira_update` | No |
| Edit description | `update` | `gojira_update` (plain text replaces it) | `e` |
| Edit due date, components, fix versions, story points | `update` | `gojira_update` | `E` |
| Comment | `comment` | `gojira_comment` | `m` |
| Log work | `log` | `gojira_log_work` | `w` |
| Assign | `assign` (by email) | **Partial**: `gojira_assign` takes an account ID only (find it with `gojira_users_search`) | `A` (by email) |
| Transition by status name | `move` | `gojira_move` | `t` |
| Close or reopen by status category | `close`, `reopen` | No (use `gojira_move`) | `X` |
| Delete | `delete --confirm KEY` | `gojira_delete` (preview plus single-use token) | `D` (type the key) |
| Move to a sprint | `sprint move` | No | `S`, `a` |
| Move to the backlog | `backlog` | No | `b` |

MCP write tools exist only when the server starts with `--allow-writes`.

## Reports and people

| Capability | CLI | MCP | TUI |
| --- | --- | --- | --- |
| Search users | `users search` | `gojira_users_search` | No |
| Time report by person and date range | `report time` | `gojira_report_time` | No |

## Different on purpose

These are not gaps: they serve the surface they live on.

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
