# Issue 23 — Agent-first access design
Design only; no runtime implementation or dependency changes.
Branch: `feature/issue-23-agent-first-access`. Commit: not authorized.

## Specs

- S1: "Project context — resolution order: flag > environment variable > `git config gojira.project` (read with `git config --get`, never by parsing the file, so worktrees and submodules work) > user-level config."
- S2: "`gojira context --json` — reports the resolved project, where it came from (flag, env, git, user) and the authenticated identity."
- S3: "MCP server (`gojira mcp`, stdio)"; "Defaults: read-only; write tools opt-in; no `delete` tool in the first version; clear tool annotations (they are hints, not guarantees)."
- S4: "Optional agent guide, per user"; "Never a tracked file and never an `AGENTS.md` edit."
- S5: "Reporting capabilities for the PM use case — user lookup by name, and time/ticket reports by person, project and date range with explicit completeness (see #22)."
- S6: "PM onboarding/distribution — a per-user token is high friction for a non-developer; OAuth 3LO (#4) would help."
- S7: "Gojira is optional and personal per developer/user, so it must be fully usable with zero artifacts, and it must never create tracked files in a repository"

## Tasks

- [x] T1 (S1–S7, route: inline design, commit: none authorized): inspect repository and official external contracts; evidence and limitations below.
- [x] T2 (S1–S7, route: inline documentation, commit: none authorized): propose contracts, delivery slices, acceptance checks and non-goals below. Proposals are not implementation approval.
- [x] T3 (S1–S7, route: memory, commit: n/a): mirror the design in Engram.
- [x] T4 (S1–S7, route: GitHub comment, commit: n/a): published and read back https://github.com/yepizrene-devoost/gojira/issues/23#issuecomment-6253032901.

## Design

### Boundaries and observed baseline

Issue #22 has landed on develop (8c23e5b): explicit search completeness is available. Existing CLI JSON arrays remain unchanged; new commands may introduce versioned object schemas. `cmd/config.go` already stores `default_project`, but commands do not use it. `gojira config init` configures credentials; the proposed top-level `gojira init` configures project context, not credentials. `cmd/projects.go` calls GetProjects, currently using a fixed maxResults=100 endpoint: a reliable picker needs pagination before claiming to show every project. There is no MCP dependency or server. User resolution is currently email-oriented. Detail worklogs are not an aggregate reporting interface.

The no-artifacts rule concerns installed product behavior, not this explicitly requested repository design document. No tracked workflow, configuration or AGENTS.md edits will be created by the product.

### S1–S2: project context

Proposed variable: `GOJIRA_PROJECT`. Resolve flag > environment > Git's `gojira.project` > existing user `default_project`. Empty/unset values fall through; invalid nonempty values fail explicitly rather than silently falling back. Use the Git executable, never parse .git (which can be a file).

`git config --get gojira.project` honors Git's own scopes, including global config, includes and worktree overrides. Consequently source `git` means Git-resolved configuration, not necessarily the current clone's local file; include its scope in diagnostic metadata if available. Do not change global Git configuration or enable worktreeConfig. `gojira init --project KEY --yes` writes with `git config --local`; linked worktrees share that clone-local setting unless an existing worktree override takes precedence. Surface that override rather than promising it was changed. A non-repository init fails without writing user config; repo-free PM usage relies on explicit input or user config.

TTY init offers a paginated project list and confirmation. Non-TTY init requires the explicit key and --yes, validates project accessibility before saving, and never prompts. No credentials in Git config. Keep `search` explicit JQL; no implicit project injection.

Proposed success response for `context --json`:

```json
{"schemaVersion":"v1","project":{"key":"DEMO","source":"git"},"identity":{"accountId":"example","displayName":"Example User"},"needsProject":false}
```

Without project: `project:null`, `needsProject:true`, successful context discovery (exit 0), allowing an agent to ask rather than guess. Authentication/network failures use the existing error contract, not a fabricated identity. Resolve identity through the authenticated-user endpoint, to be centralized in paths.go and verified during implementation. Return no tokens; avoid email unless actually needed. Context discovery is not a JQL command.

Acceptance: precedence table, empty/invalid values, ordinary repos/submodules/linked worktrees, existing worktree overrides, no-repo execution, non-TTY refusal and zero-write failures. Preserve config init and existing search behavior.

### S3: MCP stdio

Use the official `github.com/modelcontextprotocol/go-sdk/mcp`; pin a compatible release during implementation, not the moving main branch used for API research. `mcp.NewServer`, `mcp.AddTool`, `mcp.StdioTransport`, `ServerOptions.Instructions` and AddResource are documented APIs.

Share application operations and serializers with CLI, not shell execution or captured CLI output. Stdout carries only MCP frames; stderr carries sanitized diagnostics. CLI error codes/hints/mutation states retain their meaning, but MCP transports them as tool result structuredContent with isError, not raw CLI stderr or process exit codes. Invalid tool arguments/protocol errors remain distinguishable from Jira failures. Avoid a global Cobra flag/error refactor merely to add a server.

Proposed first tool set: `gojira_context`, `gojira_projects`, `gojira_boards`, `gojira_get`, `gojira_search`, `gojira_sprints`. Search takes explicit JQL and a bounded limit; every collection exposes completeness. Get must not present partial embedded comments/worklogs as complete history. Exclude export filesystem writes from read-only tools. No arbitrary REST, shell or filesystem tool. Configure credentials once using existing resolution; server cwd does not identify a PM's project. No implicit client roots inspection or repository scanning.

Read tools: readOnlyHint=true, destructiveHint=false, idempotentHint=true, openWorldHint=true. These are documentation hints, not authorization. Server instructions plus a `gojira://guide` static resource explain explicit JQL, missing project, completeness, permission errors and untrusted ticket text. The guide is available without installation artifacts.

Acceptance: real stdio handshake/list/call/resource lifecycle, JSON schema validity, malformed input, permission mapping, no stdout contamination, pagination/truncation, disconnect/cancellation propagation, bounded requests and no registered mutation tools by default. No mandatory concurrency optimization in v1.

### S4: optional guide

Default: install nothing. MCP instructions/resource is the repo-free route. A later explicit guide-install command may target a user-selected per-user skill location, or a repo-local file excluded using .git/info/exclude. Preview changes, never overwrite without consent, never edit AGENTS.md. Client-specific skill formats remain adapters, not a required Gojira runtime dependency.

Acceptance: no-file default, opt-in path, exclusion behavior, existing file refusal, no tracked artifacts and no secrets in the guide.

### S5: lookup and reporting

Proposed `users search QUERY --json`: return candidates with accountId/displayName/active, handle ambiguity explicitly; never select a person by displayName alone. Official user-search endpoint permissions and privacy behavior must be verified before that slice is implemented; current email assignment resolution is not reused as a name-identity guarantee.

Proposed `report time --account-id ID --project KEY --from YYYY-MM-DD --to YYYY-MM-DD --timezone IANA --json`. Human dates are inclusive; convert to a half-open interval [start-of-from, start-of-day-after-to) in the explicit timezone. Filter individual worklogs by author accountId and started instant; aggregate integer seconds, not rounded hours. Date-boundary/DST cases require tests. A ticket report is separately defined as matching tickets, not a claim about historic assignees or worklog authors.

Primary conservative implementation: paginate explicit project-scoped issue search, then paginate `GET /rest/api/3/issue/{issueIdOrKey}/worklog`, filtering worklogs locally. JQL worklogAuthor/worklogDate can narrow candidates only as a separately validated optimization: issue-level predicates do not prove that the same worklog matches every predicate, and indexing/timezone limitations can cause omissions. Never sum issue timespent or only embedded worklogs as the requested period/person total.

Verified alternatives: `GET /rest/api/3/worklog/updated?since=...` lists IDs by update time (not started time), paginated, excluding the preceding minute; `POST /rest/api/3/worklog/list` fetches up to 1000 known IDs. These suit incremental synchronization, not a stateless started-date report without a complete baseline. Do not use the internal ASAP-only bulk endpoint. No local database/sync architecture in the initial report slice.

Proposed metadata: `schemaVersion`, `filters`, `rows`, `totalSeconds`, `complete`, `completenessScope:"visible_to_authenticated_user"`, `issuesScanned`, `worklogsScanned`, `warnings`. Complete means every page within declared visible scope was consumed, not all organization data or a transactionally consistent snapshot. Budget/cap/rate-limit/page failures must set complete=false and make totals explicitly partial; permission gaps cannot be silently converted to zero. No invisible-data claims. Request/page budgets must be finalized with measured fixtures before implementation. The agent app generates PDFs; Gojira produces data only. Tempo-specific reporting is excluded.

Acceptance: >1 issue page and >1 worklog page, mixed authors/dates in one ticket, ambiguous user, timezone/DST, invisible worklogs disclosure, failed page, cap reached, no totals presented as complete when partial. Live verification requires a separately authorized test dataset; no private ticket text in artifacts.

### S3 guarded writes (later slice)

Server startup opt-in (proposed --allow-writes); a fixed allowlist registers create/comment/log/assign/move/update only when enabled. No delete in v1. Client confirmation is required product guidance but cannot be guaranteed by annotations or natural-language instructions. Enabling writes is broad authority: do not market it as per-call approval enforcement. If server-enforced per-action confirmation is required, design that additional approval mechanism before exposing mutations.

Treat Jira descriptions/comments as untrusted data, never executable instructions. Prefer comment/append to replacing rich ADF; expose a preview and warn about formatting loss for replacement. Propagate applied/not_applied/unknown exactly; never retry unknown writes automatically. Document tool-specific idempotence honestly (create/comment/log are non-idempotent). Read-only mode must reject writes structurally, not only through prompts.

### S6: distribution and onboarding

First desktop route: local stdio binary and a documented launch configuration; no hosted server. Existing API-token auth remains supported. OAuth browser consent/refresh/keychain remains issue #4, not an unrequested implementation in this design.

MCPB supports binary servers, platform constraints/overrides and sensitive user_config fields. A Go binary bundle is viable as a later platform-specific packaging slice; do not promise universal one-click support in Codex or every desktop client. Verify each target client's bundle support and binary signing/install requirements during that slice. Sensitive configuration support is a client feature, not proof that every client uses the OS keychain. Never embed credentials in a distributed bundle.

Acceptance: clean-machine launch per supported platform/client, credential masking, unsupported platform refusal, upgrade/uninstall instructions and a repo-free PM walkthrough. OAuth acceptance belongs to #4.

### Delivery order and review boundaries

1. Read-only MCP using existing explicit inputs and completeness contracts. No context/init dependency; context tool can wait for slice 2.
2. Context resolver + context + init + complete project picker; connect the context MCP tool.
3. User lookup, then time/ticket report with a specified completeness contract.
4. Guarded MCP writes after confirming the approval boundary.
5. Optional guide and client distribution; OAuth through #4 when needed.

Each slice requires separate implementation authorization and reviewable units with its tests/docs. This issue's deliverable is this plan; it does not authorize all slices, staging, native freeze, commits, push, merge or closure. Do not change issue labels to imply implementation approval.

### Sources and evidence limits

Official pages consulted during design:

- https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-worklogs/ — paginated issue worklogs, updated IDs, list-by-ID, permissions and visibility.
- https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-other-operations/ — internal bulk endpoint is not a normal user API.
- https://github.com/modelcontextprotocol/go-sdk — official SDK and stdio example.
- https://go.sdk.modelcontextprotocol.io/server/ — tools, structured output and resources.
- https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/mcp/server.go — ServerOptions.Instructions.
- https://modelcontextprotocol.io/specification/2026-07-28/server/tools — annotations are untrusted hints.
- https://git-scm.com/docs/git-config — Git scopes and worktreeConfig overrides.
- https://raw.githubusercontent.com/anthropics/mcpb/main/MANIFEST.md — binary type, platform compatibility and sensitive configuration.

Research establishes design feasibility, not execution evidence. No SDK integration, Jira report, OAuth flow, desktop bundle or live mutation was tested. Latest SDK release, name lookup permissions, exact date-boundary behavior of server-side worklog filters, target desktop compatibility and packaging requirements remain slice-level verification gates. No code tests run for this documentation-only design.

## Authorized implementation: first slice

User authorization: "¿Autorizas implementar la primera entrega, **`gojira mcp` de solo lectura**, dejando los cambios sin stage ni commit para tu revisión?" → "si".

- [x] M1 (S3, route: inline): official SDK v1.8.0 pinned; RED observed: `go test -mod=mod ./cmd -run '^TestMCPReadOnlyProtocol$' -count=1` failed with undefined newMCPServer, exit 1.
- [x] M2 (S3, route: inline + bounded writer after context backstop): stdio MCP adapter, five read tools, shared curated output, conservative completeness metadata, CLI help and README implemented. Focused MCP tests and full Go suite passed in writer checks; public stdio subprocess handshake passed. Context/init and writes excluded.
- [x] M3 (S3, route: independent verify; explicit user exception): protocol checks and initial full suite passed; independent verifier observed MCP race tests, vet, build and full suite exit 0. Final focused MCP tests passed after the explicit-zero fix and completeness clarification. Live stdio Jira reads and the user's Claude-client smoke test passed. Missing-credentials startup stdout proof is deferred with explicit user approval: "si lo dejamos pendiente y avanzamos". macOS keyring directly invokes /usr/bin/security, preventing PATH isolation. No vulnerability advisory audit performed. User authorized stage/freeze and commit only if RDD approves; no push, merge or closure authorized.

Risk: item 2 (read-only tool/credential boundary) and item 5 (new SDK dependency). Keep existing CLI contracts unchanged.

Verification evidence: independent verifier reported `go test -race ./cmd -run MCP -count=1`, `go vet ./...`, `go build ./...` and `go test ./...` all exit 0 before the final boundary fix. It flagged ambiguous collection completeness. Final implementation exposes `completeness:unknown` for one-page listings/get, and `complete`/`partial` for search; existing Jira APIs remain unchanged. Worker reproduced explicit `limit:-0` succeeding through public CallTool (RED), fixed presence-based defaulting, and ran `go test ./cmd -run TestMCP -count=1` (GREEN). Parent read final adapter/tests and synchronized README. No repeat full suite after the narrow fix; final focused suite covers it. Stdio subprocess handshake is covered, but missing-credentials CLI startup stdout isolation was not established safely without possible OS-keychain access. User explicitly accepted deferring this proof; it is not a claimed pass. Native read-only assessment was unassessable because untracked paths need declaration; no inspect/START/freeze was requested or run.

## Log

L1: "en esta sesión haremos el issue 23"

L2: User selected "Las 6 líneas, en un solo documento de diseño" and "Documento ODD en odd/tasks/ + issue actualizado". Deliverable is design, not runtime implementation. No commit authorized.

L3: "continua"

L4: dflow initially refused to prompt in a non-interactive terminal; rerun with --no-push created the local feature branch. No files were changed on develop.

L5: "dale". Clarification: "¿Autorizas implementar la primera entrega, **`gojira mcp` de solo lectura**, dejando los cambios sin stage ni commit para tu revisión?" User: "si". Only the first slice is authorized.

L6: "muy bien lo veo bien puedes congelar el arbol rdd y si aprueba commit". After requesting isolated missing-credentials proof ("pruebalo"), user accepted its safe-isolation limitation: "si lo dejamos pendiente y avanzamos". Pre-freeze declaration: prepare this first read-only MCP work unit for native RDD; commit is conditional on native approval. Verdict, lineage and resulting commit identity will be recorded externally, not by modifying this frozen document. No push, merge or issue closure authorized.
