# Worklog management (edit and delete) across CLI, MCP and TUI

Issue: https://github.com/yepizrene-devoost/gojira/issues/44 — branch `feature/issue-44-worklog-management`.
Scope: read the worklog id, update and delete one worklog from `gojira log`, MCP `update_worklog`/`delete_worklog`, and the TUI.

## Specs

**S1 — Worklog id in the read contract.** Every worklog the client returns carries Jira's worklog id as the string field `id`. `log KEY --show --json` emits `[{"id":"123","author":{"displayName":"Ada"},"comment":"Worked","timeSpent":"1h","started":"..."}]` and the `worklogs` array of `get KEY --json` does the same. The field is additive: entries without an id emit no `id` key, and every existing field keeps its name and meaning.

**S2 — Client update.** `(*jira.Client).UpdateWorklog(issueKey, worklogID, timeSpent, comment string) error` sends `PUT /rest/api/3/issue/{issueKey}/worklog/{worklogID}`.
- Both empty: no request, error `update requires timeSpent or comment`.
- Only `timeSpent` set: body `{"timeSpent":"3h"}`; the comment is not sent, so Jira keeps it.
- Only `comment` set: body `{"comment":<ADF>}`; the duration is not sent.
- Both set: body carries `timeSpent` and the comment as an ADF document built with `TextToADF`.
- A blank comment never travels as `""`.

**S3 — Client delete.** `(*jira.Client).DeleteWorklog(issueKey, worklogID string) error` sends `DELETE /rest/api/3/issue/{issueKey}/worklog/{worklogID}` with no body and returns the mutation error unchanged.

**S4 — CLI update.** `gojira log KEY --id ID [--time DURATION] [--comment TEXT]` updates one worklog:
- `--id` with neither `--time` nor `--comment`: exit 1, code `validation_error`, `--time or --comment is required with --id`.
- `--id` with `--time`: `✓ Updated worklog ID on KEY`; with `--comment`, the text is sent as ADF.
- `--id` plus `--json`: the existing mutation JSON contract (`writeMutationResult`) is reused.
- Without `--id`, `log` keeps creating worklogs exactly as today.

**S5 — CLI delete.** `gojira log KEY --id ID --delete --confirm ID`:
- `--delete` without `--id`: exit 1, `validation_error`, `--delete requires --id`.
- `--confirm` missing or different from `--id`: exit 1, `validation_error`, `--confirm must repeat the worklog id`, and no HTTP request is made.
- `--delete` combined with `--time` or `--comment`: exit 1, `validation_error`, `--delete cannot be combined with --time or --comment`, and no HTTP request is made.
- Valid: `DELETE` on the entry path, then `✓ Deleted worklog ID on KEY`.

**S6 — MCP `update_worklog`.** Tool arguments: required `key`, `worklogId`; optional `timeSpent`, `comment`. With neither optional argument the call fails validation with `one of timeSpent or comment is required` and requests nothing. `comment` is plain text converted to ADF; `timeSpent` is validated by the existing duration rule. The response reuses the write result envelope.

**S7 — MCP `delete_worklog`.** Tool arguments: required `key`, `worklogId`; optional `confirmationToken`. A call without a token returns a preview with a single-use `confirmationToken` and performs no request; the token expires after two minutes; an unknown, expired or reused token is rejected without a request. A call with a valid token issues `DELETE` on the entry path.

**S8 — TUI worklog list.** In ticket detail, `W` (Shift+W) opens `⏱ Worklogs: KEY` listing each entry as `id`, `timeSpent`, author and flattened comment; with none it shows `No worklogs to manage`. `↑`/`↓` move the selection, `e` opens the edit form, `d` opens the delete confirmation, `esc` returns to the detail modal. The hint line reads `[↑↓] Select  [e] Edit  [d] Delete  [esc] Back`.

**S9 — TUI worklog edit.** `e` opens `✎ Worklog: KEY #ID` with the current duration and comment prefilled; `Tab`/`Shift+Tab` move between fields, `Ctrl+S` submits once, `esc` cancels without writing. A blank duration is rejected inline and sends nothing. On success the flow returns to the detail modal, refetches the issue and toasts `Worklog updated`; a late reply for a cancelled request changes nothing.

**S10 — TUI worklog delete.** `d` opens `🗑 Delete worklog KEY #ID` showing the duration; `y` deletes, `esc` cancels. On success the flow returns to the detail modal, refetches the issue and toasts `Worklog deleted`; any other key is ignored.

**S11 — Docs.** README lists the new CLI flags, the TUI keys and flow, the MCP tools, and the `id` field in the `log --show --json` field table, in the same work unit as the behavior.

**S12 — Post-write verification.** After a successful worklog update the client re-reads that worklog and reports when Jira did not keep the requested duration:
- `(*jira.Client).VerifyWorklogUpdate(issueKey, worklogID, wantTimeSpent string) error` returns nil when the reported duration equals `wantTimeSpent`, or when `wantTimeSpent` is empty; otherwise the error reads `jira reports 15m for worklog 123 (requested 30m)`.
- It performs one read of the issue worklogs and never mutates anything.
- CLI: a mismatch prints `warning: <error>` to stderr after the success line and keeps exit code 0, because the mutation itself was accepted.
- TUI: a mismatch returns to the detail modal with the success toast and shows the warning as the modal error.
- MCP: a mismatch adds the warning to the write result `hint`, keeping the applied state.

## Tasks

- T1 — Foundation (S1, S2, S3): `internal/jira/types.go`, `internal/jira/paths.go`, `internal/jira/client_worklog.go`, tests. **done** — `go test ./internal/jira/` green; new tests `TestGetWorklogExposesID`, `TestUpdateWorklogSendsOnlySuppliedFields`, `TestUpdateWorklogRequiresAChange`, `TestDeleteWorklogSendsEntryPath`, `TestTicketJSONWorklogsExposeID`, `TestTicketJSONWorklogWithoutIDOmitsField`. commit: 4bc0f50.
- T2 — CLI (S4, S5): `cmd/log.go`, `cmd/log_test.go`. **done** — `go test ./cmd/ -run TestLog` green, `go vet ./cmd/` clean; rejected input makes no request and prints nothing to stdout. commit: b9806c6.
- T3 — MCP (S6, S7): `internal/mcpserver/writes.go`, `writes_validation.go`, `write_confirmation.go`, `writes_surface_test.go`, `writes_worklog_requests_test.go`. **done** — `go test ./internal/mcpserver/` green; token binding generalized to `action:key[:worklogId]` so a `delete` token cannot redeem on a worklog. commit: 0004f15.
- T4 — TUI (S8, S9, S10): `internal/tui/tui.go`, `internal/tui/tui_test.go`. **done** — `go test ./internal/tui/ -run TestWorklog` green; views `viewWorklogList`/`viewWorklogEdit`/`viewWorklogDelete`, `W` entry point, `reloadDetail` refetch. commit: 8160460.
- T5 — Docs (S11): `README.md`. **done** — CLI `log` row, TUI `W` row and flow paragraph, MCP tool table, additive `id` in both worklog JSON descriptions, parity matrix row, token/confirmation paragraph. commit: a607481.
- T6 — Verification: **done** — `go build ./...` OK, `go vet ./...` OK, `go test ./... -count=1` zero failures (14 packages), `gofmt -l` clean on every touched file except the two `internal/tui` files that were already unformatted at HEAD. No separate commit: verification accompanies each slice.
- T7 — Post-write verification (S12): **done** — client `VerifyWorklogUpdate`, CLI stderr warning with exit 0, TUI warning preserved through the detail refetch, MCP `hint` with state kept `applied`. commits: 4bc0f50, b9806c6, 0004f15, 8160460.
- T8 — CLI flags (S13, S14b): **done** — `--show`/`--id` rejected first, `--clear-comment` with its conflicts, and the combined `--time --clear-comment` reduced to one request by the parent after the two-request version was caught by a new test. commit: b9806c6.
- T9 — MCP clearing (S14d): **done** — `clearComment` boolean, one request for both combinations, `one of timeSpent, comment or clearComment is required`; the parent updated the schema pin in `writes_surface_test.go` that the worker could not touch. commit: 0004f15.
- T10 — TUI clearing and complete list (S14c, S15): **done** — `W` loads the full collection with a scoped request, loading state and in-list error; the emptied prefilled comment is saved as one PUT with a null comment. commit: 8160460.

## Log

- L1 (user, verbatim): "ademas no seria bueno agregar la opción de modificar un worklog? o eliminarlo?"
- L2 (user, verbatim, asked how to proceed): "crear el issue e implementar por completo ahora mismo"
- L3 (user, verbatim): "bueno cerramos esta rama con dflow finish --delete y abrimos otra para el issue"
- L4 (decision): the ADF worklog fix was committed as `f670dcd` ("sin rdd") and delivered with `dflow finish --delete`: merged into `develop`, branch deleted locally and remotely. This feature starts from `develop` on `feature/issue-44-worklog-management`.
- L5 (decision): issue 44 created with `status:approved` because the user approved full implementation; no live Jira verification is available in tests, so wire payloads are pinned with `httptest` captures instead.
- L6 (decision): TUI gains a selection list rather than editing a worklog from the detail body, because the detail modal has no field focus of its own and the codebase keeps one plain string-state view switch.
- L7 (evidence, parent): the CLI MCP-surface test `cmd/mcp_writes_test.go` pinned the tool count at 21; with `update_worklog` and `delete_worklog` the surface is 23, so that expectation was updated in the same unit. `internal/jira/types.go` also needed a gofmt alignment fix after adding `Worklog.ID`.
- L8 (known limitation, open): clearing the comment field in the TUI or passing `--comment ""` leaves the Jira comment unchanged, because an empty comment is omitted from the PUT (same rule that fixed the 400). Removing a comment would need an explicit empty ADF document; not specified by this feature.
- L9 (known limitation, open): `log KEY --show --id ID` hits the update validation before the history branch, because the spec does not define that combination.
- L10 (user, verbatim): "Añadir verificación posterior a la escritura" — chosen to handle worklogs owned by Tempo: "Tras el PUT, gojira relee el worklog y avisa si Jira no conservó el valor (detecta ediciones ignoradas), sin intentar hablar con Tempo."
- L11 (evidence, live Jira): after editing a Tempo-authored worklog from the TUI, `fields.worklog` and `fields.timetracking.timeSpent` both returned 30m, so the PUT applied in Jira; the browser's Tempo panel still showed 15m because Tempo keeps its own records. S12 exists to detect the case where Jira itself ignores the change.
- L12 (evidence, parent): the detail footer grew past one line when `[W] Manage` was added, splitting `[esc] Close`; it now lives on the second footer line. A test pins the footer entry.
- L13 (decisions, T7): the MCP surface reaches the check through a type assertion on `Writer` instead of widening the interface, because the fakes live outside that unit's surfaces; a writer without the method skips verification silently and the real `*jira.Client` has it. The TUI also had to stop `detailMsg` from clearing `m.err` so the warning survives the refetch that follows the mutation.
- L14 (handoff): everything stays uncommitted in the working tree for the maintainer's review; no stage, no push, no `dflow finish`. Checks at handoff: `go build ./...` OK, `go vet ./...` OK, `go test ./... -count=1` zero failures, `gofmt -l` clean on every touched file except the two `internal/tui` files that were already unformatted at HEAD.
- L15 (user, verbatim): "si y aparte la implementación de la corrección del apartado anterior que mencionabas" — approving the list-truncation fix plus the two open corrections (explicit comment clearing and a clear rejection of `--show --id`), all before any commit.
- L16 (evidence, live Jira): Tempo's `Worklogs / Plans` tab listed two entries (15m and 10m) while Jira's embedded `fields.worklog` carried one (`#29399`, 20m, author "Timesheets by Tempo - Jira Time Tracking"). Jira's REST API does not proxy Tempo's records; Tempo has its own API and product. The embedded page is also not the full list, which is what S15 addresses.
- L17 (open risk): clearing a comment sends `"comment": null`. Jira's documented behaviour for null on a worklog update is not verified against a live site from this repository; the tests pin the wire payload only.

## Specs S13–S15 (added after L15)

**S13 — `--show` and `--id` never mix.** `gojira log KEY --show --id ID` exits 1 with code `validation_error` and the message `--show cannot be combined with --id`, before any request; both flags keep their current behaviour alone.

**S14 — explicit comment clearing.** Removing a worklog comment becomes a deliberate act instead of an empty field:
- `(*jira.Client).ClearWorklogComment(issueKey, worklogID string) error` sends `PUT` on the entry path with exactly `{"comment":null}`.
- CLI adds `--clear-comment`: it requires `--id` (`--clear-comment requires --id`), cannot be combined with `--comment` (`--clear-comment cannot be combined with --comment`) or with `--delete` (covered by the existing delete conflict message), and on success prints the update line.
- TUI: the edit form is prefilled with the current comment, so saving it empty clears the comment; the flow still returns to the detail modal with `Worklog updated`.
- MCP: `update_worklog` accepts an optional boolean `clearComment`, rejects it together with a non-empty `comment`, and counts it as content, so the empty-request message becomes `one of timeSpent, comment or clearComment is required`.

**S15 — complete worklog list.** `W` loads every worklog from `GET /rest/api/3/issue/{issueKey}/worklog?startAt=0&maxResults=1000` instead of the page embedded in the ticket read: it shows `Loading worklogs...` while fetching, lists what Jira returns, keeps entries selectable, and surfaces a read error inside the list instead of showing a stale page.

## Post-T10 decisions

- L18 (decision, parent): the combined "change duration and clear comment" case is one `PUT` everywhere. The client exposes `UpdateWorklogAndClearComment`; the CLI no longer issues two mutations, the MCP tool uses the same call, and the TUI's emptied prefilled comment goes through it.
- L19 (decision, parent): the MCP schema pin in `writes_surface_test.go` belongs to the surface that registers the tool, so the parent updated it instead of widening a worker's edit surface.

## Commit slices

- C1 `feat(jira)`: `internal/jira/*` plus this document.
- C2 `feat(cli)`: `cmd/log.go`, `cmd/log_test.go`.
- C3 `feat(mcp)`: `internal/mcpserver/*`, `cmd/mcp_writes_test.go`.
- C4 `feat(tui)`: `internal/tui/tui.go`, `internal/tui/tui_test.go`.
- C5 `docs`: `README.md`.
- C6 `docs(odd)`: commit identities and this slice map.

Slice boundary rule: each slice is one cohesive behavior with the tests that pin it. C2 and C4 exceed the ~400-line review guidance because their tests travel with the rules they pin, and splitting them would separate a rule from the test that proves it.
