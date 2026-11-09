# Comment management across CLI, MCP and TUI (issue #39)

Feature: complete comment listing with pagination, read/edit/delete by comment
ID across all three surfaces, preserving existing add-comment behavior.
Issue: https://github.com/yepizrene-devoost/gojira/issues/39 (status:approved).
Branch: feature/issue-39-comment-management (from develop).

## Specs

- S1: "Add complete comment listing with explicit pagination/completeness, and
  read, edit and delete by stable comment ID across CLI, MCP and TUI; preserve
  the existing add-comment behavior."
- S2: "Define accessible discovery and selection of comment IDs and display
  author, timestamps and appropriate content. Preserve existing JSON contracts
  and stdout/stderr separation."
- S3: "Handle Jira ADF deliberately: do not silently destroy rich formatting,
  mentions or visibility restrictions when editing; establish and document
  supported editing semantics before implementation."
- S4: "Treat deletion as destructive: provide explicit confirmation in CLI and
  TUI and a suitable preview/confirmation flow in MCP. A tool token is not
  proof of human consent. Honor Jira permissions and report applied,
  not_applied or unknown outcomes without automatic write retries."
- S5: "Use strict test-first RED/GREEN and isolated Jira fakes, including
  permissions, pagination, validation, rejected writes and mutation
  uncertainty. Update help and documentation across surfaces."
- S6 (issue, context): "Existing entry points: cmd/comment.go,
  internal/jira/paths.go, internal/jira/types.go, internal/mcpserver/writes.go
  and internal/tui/tui.go. Jira Cloud documents comment read, update and delete
  and separate own/all permissions."

Design decisions (implementation decisions, documented per S3 before code):

- D1 ADF editing semantics: editing a comment replaces its body with a new
  plain-text ADF document (same `TextToADF` used by add). Rich formatting,
  mentions and visibility restrictions are NOT round-tripped; replacing a rich
  body loses them. Every edit path warns before/at the replacement: CLI on
  stderr, TUI in the edit form header, MCP in the tool description and preview
  message. No read-modify-write of the original ADF.
- D2 CLI shape (mirrors `log`): `gojira comment KEY --list` (full paginated
  listing with total), `gojira comment KEY --id ID` (read one), `gojira comment
  KEY --id ID --text "..."` (edit), `gojira comment KEY --id ID --delete
  --confirm ID` (delete). `gojira comment KEY text` unchanged.
- D3 JSON: additive only. `TicketJSON.CommentJSON` gains optional `id` (same
  precedent as `WorklogJSON.ID`). New `--json` outputs for list/read/edit/
  delete are separate documents on stdout; warnings on stderr.
- D4 MCP: new read tool for the paginated comment list; new write tools
  `update_comment` and `delete_comment` in the writes table with token binding
  generalized to `action:key:commentId` (existing `action:key[:worklogId]`
  shape extended).
- D5 TUI: single-model constraint. New comment management view reachable from
  the issue detail (`C`), list/select/edit/delete with `y` confirmation,
  reusing the worklog management message/request-id pattern.

## Tasks

- T1 (S1,S3,S5) Client layer: comment entry path + ID escaping, decode
  comment `id`/`total` additively, `ListComments` (paginated), `GetComment`,
  `UpdateComment`, `DeleteComment`. Tests: `internal/jira/client_comments_test.go`
  (new, httptest fakes). Route: inline.
- T2 (S1,S2,S4,S5) CLI: comment list/read/edit/delete flags with
  mutual-exclusion validation before any request, `--confirm` echo for delete,
  `--json` documents, stderr warnings. Tests: `cmd/comment_test.go` (new).
  Route: inline.
- T3 (S1,S2) MCP read tool for paginated comments. Tests:
  `internal/mcpserver/tools_test.go` additions. Route: inline.
- T4 (S1,S4,S5) MCP write tools `update_comment`/`delete_comment`: validation,
  confirmation token with commentId binding, outcome reporting (applied /
  not_applied / unknown), no auto retries. Tests: `internal/mcpserver`
  request/confirmation tests. Route: inline.
- T5 (S1,S4,S5) TUI: comment manage view (list, edit prefill + ADF warning,
  delete + `y` confirm), reload detail after mutation, help/footer text.
  Tests: `internal/tui/tui_test.go` additions. Route: inline.
- T6 (S2,S5) Docs: README command table, TUI keys, MCP tool table, JSON
  contract section, `docs/interface-parity.md` comment rows. Route: inline.

## Log

- L1 (user, verbatim): "implementemos el issue #39"
- L2 (user): "para la implementación no delegues" — implement inline, no
  subagent workers; subagent used only for read-only exploration mapping.
- L3: Exploration handoff mapped existing surfaces (client: AddComment only,
  no id decode, no list/update/delete; CLI add-only; MCP add-only with
  confirmation model ready; TUI add via `m`, worklog flow as template).
- L4: Repo rules: strict TDD mandatory; work stays uncommitted until the
  maintainer has seen it; each commit needs its own explicit instruction.
- L5: T1 done — RED observed (undefined ListComments/GetComment/UpdateComment/
  DeleteComment/commentEntryPath), then GREEN: paths.go commentEntryPath,
  additive `id`/`updated` on Comment, StartAt/MaxResults/Total on CommentField,
  additive CommentJSON.ID, ListComments (monotonic page walk, page cap),
  GetComment, UpdateComment (ADF PUT), DeleteComment in client_comments.go;
  `go test ./internal/jira/` green.
- L6: T2 done — RED observed (invalid command flags / missing handlers), then
  GREEN: comment KEY gains --list/--id/--text/--delete/--confirm with all
  intent validation before the client is built; add path preserved; edit warns
  on stderr (never stdout) with the ADF replacement semantics; `--id` alone
  reads. MCP surface golden updated with `-update` (gojira_comments entry).
- L7: T3 done — RED observed (gojira_comments not registered), then GREEN:
  read tool in tools.go returning `{issueKey,total,comments}` marked complete;
  surface/parity counts updated (24 tools with writes, 10 read-only).
- L8: T4 done — RED observed (missing action update_comment/delete_comment),
  then GREEN: Writer gains UpdateComment/DeleteComment; destructive token
  binding generalized to action:key:commentId (delete/delete_worklog/
  delete_comment share one store, previews name commentId); validation adds
  trimmed/non-empty commentId and non-empty text; positional writeTools()
  indices in tests replaced by writeToolByAction; `go test ./...` green.
- L9: T5 done — RED observed (undefined viewCommentList), then GREEN: C opens
  the comment manage view reading the full collection; e prefills the editor
  (states ADF semantics), Ctrl+S submits once with blank rejection; d + y
  deletes; mutation returns to the list, reloads it and refetches the detail;
  ADF replacement warning survives into the refreshed list; detail hint and
  kanban help advertise C; `go test ./internal/tui/` green including the
  compact-help regression.
- L10: T6 done — README (command row, JSON contract comments row, capability
  parity, MCP tool table, TUI note and management paragraph, key table) and
  docs/interface-parity.md comment rows updated.

## Commit slices

(awaiting explicit maintainer instruction; nothing committed)
