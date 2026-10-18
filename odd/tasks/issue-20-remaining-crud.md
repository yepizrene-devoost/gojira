# Issue #20 — remaining ticket CRUD and TUI parity

## Origin and boundary

This is the unimplemented remainder of [issue #9](https://github.com/yepizrene-devoost/gojira/issues/9), transferred at the user's request because #9 became too broad. The GitHub source of truth is [issue #20](https://github.com/yepizrene-devoost/gojira/issues/20). #9's approved work units remain historical evidence, not work to repeat. Issue #19 remains design-only for TUI detail UX; comments implementation belongs here. The handoff does not close #9, authorize a PR or release, or permit any real Jira deletion.

## Constraints

- Keep the single Bubble Tea model, centralized endpoints in `internal/jira/paths.go`, ADF conversion in `internal/jira/types.go`, and stable TicketJSON v1 stdout with diagnostics on stderr.
- Treat `applied`, `not_applied`, and `unknown` as distinct mutation states. Never retry an ambiguous write automatically. Only use local HTTP simulations for mutation acceptance tests.
- Split implementation into independently functional work units with tests, documentation, ODD evidence, local commit authorization, and native review when enabled. Do not treat a scope transfer as completed behavior.
- Preserve the selected product contracts from #9: Done-category children/total progress; backlog removal of future/active sprint assignments without erasing closed history; per-issue available transitions for close/reopen; permanent deletion only after explicit confirmation. Neither the decision to implement deletion nor this issue authorizes deleting a real ticket.

## Open work

- [x] I20-1: Complete the all-command JSON/error audit. Check remaining commands including `version`, write-result readback and stable stderr/exit handling; document intentional non-JSON `config`/`tui` behavior. Preserve the existing approved TicketJSON v1 and raw Board/Project array shapes. Test failures with local HTTP and failing writers.
- [x] I20-2: Show verified Done-category child progress on board cards and available JSON, plus sprint name/state on `get` and cards. Distinguish unavailable fields from genuine zero; avoid N+1 reads and do not infer undocumented Jira field shapes as fact.
- [x] I20-3: Support remaining CLI/TUI sprint selection and movement, including TUI backlog movement beyond add-to-active. Preserve request scoping, partial/unknown write states, and safe refresh behavior; closed-sprint history must not be rewritten by a synthetic multi-write loop.
- [x] I20-4: Complete CLI/TUI lifecycle parity: close/reopen through transitions actually available on the selected issue and permanent delete with a clear, separate confirmation. Test only with local HTTP; no real-ticket deletion without a new explicit action authorization.
- [x] I20-5: Parse logged time/worklogs into TicketJSON, aggregate time on `get`, and complete TUI detail with readable dates, worklog/time summary, existing comments, and comment entry. Preserve ADF and detail navigation. Coordinate interaction design with #19 without treating that design issue as implementation authority.
- [x] I20-7: Explicit, agent-readable permission/authentication errors across CLI JSON and human output (see "I20-7 Specs"). Added by user requirement L4/L5.
- [x] I20-6: Reconcile README, focused/full Go tests, local HTTP/TUI regression evidence, and the remaining CLI/TUI parity matrix before requesting delivery. Publishing, issue closure, tagging and release remain separate user decisions.

## Handoff evidence

Issue #20 was created and read back as OPEN. Issue #9 was annotated with the scope handoff and remains OPEN; issue #19's design-only references now point comment implementation here and it remains OPEN. The last approved #9 work-unit boundary was `891b896c109d0c5956522b90b40daab7624b3eed` after I9-2e (`review-edd8a4d836ca311b`, approved and acknowledged). This document only transfers future scope; it makes no claim that I20-1 through I20-6 have been implemented or reviewed.

## I20-1 evidence

Audit found read-failure envelopes already correct for every JSON command. Three real gaps fixed with RED→GREEN tests in `cmd/json_audit_test.go`: `version --json` writer failure was mislabeled `validation_error` (now `output_failed`); `move --json` and `log --show --json` printed `null` instead of `[]` for empty responses. README documents the arrays, `version` failures and the deliberate non-JSON `config`/`tui`. Full `go vet` and `go test ./...` green. Work-unit commit: `850ebd3` (`fix(json): report version write failures and empty arrays`), based on develop `be8d164111e19982d36ddd3a85a6c167a55f75de`. Native committed-only review is declared for the range `be8d164..` of this branch; verdict and approval live in the native receipt and Engram, not in this file.

## I20-2 Specs

- S1: `IssueFieldData` parses an optional `fields.sprint` object (`id`, `name`, `state`). Absent or null stays absent; never fabricate a sprint. `TicketJSON` gains an additive, omitted-when-absent `sprint` object with `id`, `name`, `state`.
- S2: `get` makes ONE extra read, `GET /rest/agile/1.0/issue/{key}` (path constant in `internal/jira/paths.go`, request `fields=sprint`; parse `fields.sprint` whether or not Jira honors `fields`). User decision L3: second Agile read is accepted. If that read fails or has no sprint, `get` still succeeds and omits the sprint; the primary issue read keeps its current error behavior. Human `get` shows a `Sprint` row (`name (state)`) only when known.
- S3: Board and sprint reads (`fieldsBasic` consumers, `GetBoardIssues`) also request `subtasks` and `sprint`, so no per-card reads happen (no N+1). One exported helper in `internal/jira` computes Done-category progress from `*[]IssueHierarchyRef` using the existing completeness rule (all children have key and a known category; done = category key `done`, never status name; `[]` is `0/0`; unknown category means no progress). `IssueToTicketJSON` reuses it, behavior unchanged.
- S4: TUI cards show progress as `done/total` (for example `2/5`) on the meta line only when the helper returns a value; no progress text otherwise (unavailable is not zero). A card shows its own sprint name only when the issue carries one and it differs from the board's active sprint name. Single Bubble Tea model, no sub-model routing.
- S5: `export` JSON now carries `subtasks`/`subtaskProgress` when Jira returns them for those reads; `search` is unchanged. Update README (TicketJSON table rows for `sprint`, progress availability, the sentence that board/export/TUI reads do not request hierarchy, and the get single-read statements) in the same unit.
- S6: Tests (RED first, then GREEN, local httptest only, no real Jira): types (sprint parse, absent/null, progress helper cases incl. `[]`, unknown category, name-vs-category), client (board/sprint read requests `subtasks` and `sprint`; agile issue read path and parse), cmd (`get --json` includes `sprint`; agile 500 still succeeds without `sprint`; request count is exactly 2 and the first is the unchanged full GET; update the existing single-GET test accordingly), tui (card shows `2/5`, none when unavailable, sprint shown only when differing). `go vet`, `go test ./...`, `golangci-lint run ./...` green.

## I20-2 Log

- L1: User: "dale" after proposing I20-2 (progress and sprint on cards and `get`).
- L2: Source issue #20 text for I20-2: show verified Done-category child progress on board cards and available JSON, plus sprint name/state on `get` and cards; distinguish unavailable fields from genuine zero; avoid N+1 reads; do not infer undocumented Jira field shapes as fact.
- L3: User chose "Segunda lectura Agile (recomendada)" for how `get` obtains the sprint (one extra GET to the Agile issue endpoint, best-effort; accepts breaking the I9-4b single-read rule for this field).

- L4: User: "puede que con mi api key en el .env no funcione como debe porque no tengo acceso a todas las funciones porque tengo permisos algo restringidos en la mayor parte de los tableros"
- L5: User: "aunque si eso lo estas haciendo revisando la docuemntación oficial de jira probablemente puedas hacer mas cosas, pero es necesario que si no hay permisos y esa tarea no se puede hacer con los permisos dar errores explicitos sobre todo que los agentes puedan leer eso para iinformar al humano que esta solicitando tareas con el tablero de jira"

## I20-7 Specs

- S1: `internal/jira` exposes a typed HTTP error (`*APIError` with `Status int` and the truncated body) returned by the request layer. Its `Error()` text stays exactly `API <status>: <body>` so existing messages and tests do not change. `errors.As` must work through wrapping, and `MutationStateOf` behavior is unchanged.
- S2: One classifier maps an error to a stable code: HTTP 401 -> `authentication_failed`, 403 -> `permission_denied`, 404 -> `not_found_or_no_permission` (Jira hides issues the account cannot see). Anything else is unclassified and keeps the command's current code.
- S3: In JSON mode the v1 stderr error envelope uses the classified code instead of the generic one (`read_failed`, `mutation_failed`, `refetch_failed`, `partial_failure` keep their `mutationState`; for `partial_failure` the classified cause is added without changing its code). It gains additive optional `httpStatus` (int) and `hint` (string, English, actionable and state-aware: states whether the change was applied, and tells the agent to report to the human that the configured Jira account lacks permission and to ask for a different board/issue or access). Existing fields and codes for unclassified errors are unchanged.
- S4: Human (non-JSON) mode prints the same hint after the existing `Error:` line on stderr for classified errors. Stdout stays untouched.
- S5: `get` stops discarding the Agile sprint read error. In JSON the success object gains an additive `warnings` array (omitted when empty) of `{code, message, httpStatus?}`; the sprint failure yields code `sprint_unavailable` carrying the classified cause in `message`. Human mode prints the warning to stderr. `get` still succeeds and exit code stays 0. No sprint is fabricated.
- S6: README documents the codes, `httpStatus`, `hint`, and `warnings`. Non-goals: TUI changes, new Jira permission pre-checks (for example `/mypermissions`), retries on 4xx.
- S7: Tests (RED first, httptest only): 401/403/404 table over `get`, `search`, `boards`, `projects`, `export`, `move` (list and `--to`), `update`, `comment`, `assign`, `log`, `backlog`, `create` (asserting exit 1, empty stdout, one envelope, classified code, `httpStatus`, non-empty `hint`, correct `mutationState`); unclassified 500 keeps its old code and has no `hint`; `get` with Agile 403 returns exit 0, valid JSON with `warnings`, and a stderr warning in human mode. `go vet`, `go test ./...`, `golangci-lint run ./...` green.

## I20-2 / I20-7 evidence

I20-2 (S1-S6) and I20-7 (S1-S7) were implemented by `gentle-ai-worker` and re-verified by the parent: `go vet ./...`, `go test ./...` and `golangci-lint run ./...` are green (0 issues). TUI gofmt-only noise from the worker was reverted so the diff keeps only behavior. The permission matrix (`cmd/permission_errors_test.go`) covers 13 commands with 401/403/404/500; `get` keeps working and reports `warnings` when the Agile sprint read is denied. All work is local and uncommitted at freeze time: a commit needs the user's explicit instruction (`AGENTS.md`, "Review Before Commit"). The review stage records only the freeze act and the declared expectation; verdict and approval live in the native receipt and Engram. No live Jira verification was performed.

## I20-3 Specs

- S1: New `gojira sprint` command group. `sprint list --board N [--state active,future,closed]` prints sprints (id, name, state) using the existing `GetSprints`-style Agile read, default states `active,future`; `--json` prints an array (`[]` when empty, never `null`) of `{id,name,state}`. `board` is required and validated as a positive integer.
- S2: `sprint move KEY --to ID` moves ONE issue into sprint ID with one `POST /rest/agile/1.0/sprint/{id}/issue` (reuse `AddIssuesToSprint`). Validate KEY with the existing key pattern and ID as a positive integer before any request. It never loops over several writes and never touches closed-sprint history. Human output states that Jira accepted the request; `--json` re-fetches the issue through `writeMutationResult`.
- S3: Errors use the existing `commandError`/envelope machinery: `validation_error`, `configuration_error`, `read_failed` (list), `mutation_failed` with `jira.MutationStateOf`, plus I20-7 classification (401/403/404 codes, `httpStatus`, `hint`). Nothing new in stdout on failure.
- S4: Endpoints only via `internal/jira/paths.go`. `backlog KEY` is unchanged.
- S5: TUI (single model, no sub-model routing): from ticket detail or the selected card, key `b` moves the issue to the backlog using `MoveIssueToBacklog`, and key `S` opens a sprint chooser listing future/active sprints of the loaded board; Enter moves the issue with `AddIssuesToSprint`. Use the existing request-id scoping pattern so a stale response after switching board/search is ignored; on success refresh the board; on error show the classified message and the applied/unknown state without auto-retry. Check existing key bindings in `internal/tui/tui.go` for conflicts and add the new keys to the help table.
- S6: README: commands table, examples, TUI key table, and the statement that closed-sprint history is not rewritten and no live Jira verification was done.
- S7: Tests (RED first, httptest only): sprint list (states filter, empty `[]`, 403 classified), sprint move (exactly one POST to the right path with `{"issues":[KEY]}`, invalid key/ID make zero requests, 500 gives `mutation_failed` not_applied, transport drop gives `unknown`, 403 classified), TUI (key opens chooser, Enter issues one POST, backlog key issues one POST, stale response ignored). `go vet`, `go test ./...`, `golangci-lint run ./...` green.

## I20-3 Log

- L6: User: "haz commit y continua" (commits da99e9b and 2b8bdeb made; reviewed tree ec214cd unchanged).
- L7: User chose design "Comando `sprint`": `sprint list --board N` and `sprint move KEY --to ID`, one POST per move, TUI backlog key and sprint chooser.

## I20-3 evidence

Implemented by `gentle-ai-worker`, re-verified by the parent: `go vet ./...`, `go test ./...`, `golangci-lint run ./...` green (0 issues). `gojira sprint list/move` (CLI, one POST per move, `[]` when empty), TUI `S` chooser and `b` backlog with request-scoped results. Spec correction: HTTP 500 on a write stays `unknown` (the original S7 line saying `not_applied` was wrong; ambiguous writes are never assumed unapplied). No live Jira verification. Left uncommitted for user review.

## I20-4 Specs

- S1: `Transition` gains optional destination data parsed from Jira's `to` object (`to.name`, `to.statusCategory.key`). Reads use `GET /rest/api/3/issue/{key}/transitions` (the existing path); never assume the field exists. Existing `move` output (`id`, `name`) and its JSON array shape stay unchanged; any new JSON properties on it are additive only.
- S2: `gojira close KEY` and `gojira reopen KEY` read the issue's available transitions and select by destination status category: close = category `done`, reopen = category `new` or `indeterminate`. Exactly one candidate: apply it with one transition POST (reuse `TransitionIssue`). Zero or several candidates: make NO write, fail with `validation_error` (not_applied) listing the candidates (id, name, destination). Missing destination data counts as "not a candidate". Never infer a workflow name. `--json` re-fetches via `writeMutationResult`.
- S3: `gojira delete KEY --confirm KEY` issues one `DELETE /rest/api/3/issue/{key}` (path constants in `internal/jira/paths.go`) only when `--confirm` equals KEY exactly and KEY matches the existing key pattern; otherwise `validation_error`, zero requests. No `deleteSubtasks` parameter (Jira then refuses to delete a parent with subtasks; report that error classified). Never retried. `--json` success prints a small object (`schemaVersion`, `key`, `deleted: true`); there is no issue to re-fetch. State is `applied`/`not_applied`/`unknown` via `MutationStateOf`, so a 5xx/transport drop is `unknown` and the hint says to verify before retrying.
- S4: All errors reuse `commandError` machinery and I20-7 classification (401/403/404 codes, `httpStatus`, hint). A 403 on delete reports `permission_denied` with the not-applied hint.
- S5: TUI (single model): from ticket detail or the selected card, `X` closes or reopens following the S2 rules (when ambiguous, show the candidates and apply nothing; the existing `t` transition picker remains the manual path), and `D` opens a confirmation that requires typing the issue key exactly, then Enter issues one DELETE. Use the existing request-id scoping; stale results are ignored; on success refresh the board and leave the detail; on error show the classified message and the state without auto-retry. Check key conflicts and update the help table.
- S6: README: command table, examples, TUI keys, an explicit warning that delete is permanent, and that no live Jira verification was done.
- S7: Tests (RED first, httptest only): close/reopen (exactly one candidate -> one POST with its id; none or several -> zero POSTs and `validation_error`; missing `to` data not a candidate; 403 classified), delete (mismatched/missing `--confirm` or bad key -> zero requests; success -> one DELETE to the right path and `deleted: true`; 403 -> permission_denied not_applied; 500 and transport drop -> unknown; subtask-parent 400 surfaced), TUI (`X`, `D` confirmation requires exact key, one DELETE only, stale response ignored). `go vet`, `go test ./...`, `golangci-lint run ./...` green.

## I20-4 Log

- L8: User: "y cotinua" (continue after I20-3 commit 79e07fb).
- L9: User chose "Categoría de estado + confirmación por clave": `close`/`reopen` by destination status category with no guessing, `delete KEY --confirm KEY`, TUI `X` and `D` with typed-key confirmation.

## I20-4 evidence

Implemented by `gentle-ai-worker` (CLI, then TUI) and re-verified by the parent: `go vet ./...`, `go test ./... -count=1`, `golangci-lint run ./...` green (0 issues). `gojira close|reopen KEY` select by destination status category and write nothing when zero or several candidates exist; `gojira delete KEY --confirm KEY` sends one never-retried DELETE only on an exact match; 5xx/transport failures stay `unknown`. TUI `X` (close/reopen, refuses when the current category is unavailable and points to `t`) and `D` (typed-key confirmation) are request-scoped. Side effect: `move --json` may carry the additive `to` object. No live Jira verification. Left uncommitted for user review.

## I20-5 Specs

- S1: `get --json` and the mutation re-fetch populate `TicketJSON.timeLogged` from Jira's documented `fields.timetracking.timeSpent` (request the `timetracking` field in `fieldsFull` only; board/search/export field sets stay unchanged). Omit when absent; never compute or guess a total from partial data.
- S2: `TicketJSON` gains an additive, omitted-when-empty `worklogs` array of `{author, timeSpent, started, comment}` built from the already requested `fields.worklog.worklogs`. Jira embeds only a bounded first page there: expose additive `worklogsTotal` (from `fields.worklog.total`) only when Jira supplies it, so consumers can tell the list may be truncated; do NOT add extra requests. `started` uses the same RFC3339 normalization as other dates. Worklog comments may arrive as ADF (v3): flatten them in `internal/jira/types.go`; verify against Atlassian's documented v3 worklog shape and, if the existing `Worklog.Comment string` cannot decode an ADF comment (this also affects `log --show`), fix it with a test and report it as a decision.
- S3: Existing TicketJSON v1 fields and behaviors are unchanged (additive only). README TicketJSON table documents `timeLogged`, `worklogs`, `worklogsTotal`.
- S4: TUI ticket detail loads and renders: readable `Created`/`Updated` (reuse the existing date normalization, shown as `YYYY-MM-DD HH:MM`), a `Time logged` line (timeSpent, plus the entries count when the list is truncated, e.g. `3h (showing 20 of 41 worklogs)`), a `── Comments ──` section with author, readable date and flattened body in Jira order, an explicit `No comments` empty state, and a worklog summary. The detail request must ask for the extra fields only for the detail view (do not widen board card reads). Long content stays scrollable with the footer visible. Keep the single Bubble Tea model and existing detail navigation/return behavior; interaction design polish belongs to issue #19 and is NOT in scope.
- S5: TUI comment entry: a key in ticket detail (check conflicts; suggest `m`) opens a multi-line input; submit sends ONE `AddComment` (plain text converted with existing helpers; mentions are a non-goal), then refreshes the detail. Request-id scoping like the other detail edits; stale results ignored; classified error plus applied/not_applied/unknown state on failure; a failed refresh after an applied write says the comment was applied and offers only a refresh retry, never a second POST. Update the help table and footer.
- S6: README: TicketJSON rows, TUI keys, detail contents. State that no live Jira verification was done.
- S7: Tests (RED first, httptest only): JSON (timeSpent present/absent, worklogs with and without total, ADF comment, date normalization, unchanged existing fields), client request field list for the detail vs board, TUI (detail renders comments, empty state, time line, truncation note, readable dates; comment entry issues exactly one POST; applied-then-refresh-failed never reposts; stale response ignored). `go vet`, `go test ./... -count=1`, `golangci-lint run ./...` green.

## I20-5 Log

- L10: User: "ok haz commit y continua" (I20-3 and I20-4 committed as 79e07fb and 5a68909).
- L11: Verified before specifying: TUI detail shows raw Created/Updated, no comments, no time; `TicketJSON.timeLogged` is never populated.

## I20-5 evidence

Implemented by `gentle-ai-worker` in three passes and re-verified by the parent: `go vet ./...`, `go test ./... -count=1`, `golangci-lint run ./...` green (0 issues); the worker's non-gofmt code in three previously clean files was formatted by the parent. `get --json` carries additive `timeLogged`, `worklogs`, `worklogsTotal`; `fields.timetracking` and the embedded worklog page/total shapes were NOT confirmed in the official Atlassian pages consulted, so they are optional and never inferred. Verified from the docs: worklog `comment` and comment `body` are ADF objects; the old `Worklog.Comment string` could not decode them, so `log --show` was likely broken against real Jira and is fixed (flattened string kept in output). TUI detail shows readable dates, time line, comments with an empty state and worklog summary; `m` opens comment entry (Ctrl+S submits, one POST, refresh-only retry after an applied write). No live Jira verification. Left uncommitted for user review.

## I20-6 evidence

README now has a CLI/TUI parity matrix, updated feature and project-structure lists. Claims were checked against the code: every CLI command except `config`/`tui` exposes `--json` (checked with `--help`), and the TUI has no assign action (only displays the assignee). `go vet`, `go test ./... -count=1` and `golangci-lint run ./...` run green on the final tree. Reviews: I20-1 `review-660fb6334152ed91`, I20-2/I20-7 `review-df91a17724b554d8`, I20-5 `review-cae5377db1cb9bd2`; I20-3 (`79e07fb`) and I20-4 (`5a68909`) were committed without native review. No live Jira verification of any command; deletion in particular is untested against a real instance. Publishing, PR, issue closure, tagging and release remain separate user decisions.
