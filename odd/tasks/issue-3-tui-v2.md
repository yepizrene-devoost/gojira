# Issue #3 — TUI v2, board loading, overlays, and search

## Objective
Upgrade the single-model TUI to Bubble Tea/Bubbles/Lip Gloss v2 and make board navigation, detail, help, and search responsive and unambiguous.

## Context and scope
- Issue: https://github.com/yepizrene-devoost/gojira/issues/3
- Branch: `feature/tui-v2-modals` from `develop`; local only until separate publication approval.
- User video: switching boards temporarily displays the previous board's sprint and cards. Verified in `internal/tui/tui.go`: selection enters Kanban without invalidating the old columns; asynchronous completions have no request identity.
- Implement: safe board-loading state and response ordering, v2 migration, floating ticket detail and help overlays, local and server ticket search, and proportional status/async polish. Preserve one Bubble Tea model, centralized Jira paths, and existing create/sprint/transition behavior.
- Optional animations and mouse support only if straightforward; never trade accessibility or correct state for visual polish. New keyboard interactions require matching README documentation.
- TDD: no configured TDD mode found in repository/session; ordinary focused behavior tests and `go test ./...` are required. Runner: `go test ./...`; runtime harness: `go run . tui` when interactive Jira is available, otherwise reducer/render tests and note unavailable live credentials.
- Delivery strategy: `ask-on-risk`; user prefers short work-unit commits around 400–500 changed lines with review of each committed candidate. This is advisory, not a reason to truncate tests or force an incoherent split. Forecast above 400 authored changed lines across migration and multiple UX surfaces; resolve chain strategy when applicable. No push, PR, merge, issue closure, or release without separate decisions.

## Tasks
- [x] T3-1 — Prevent stale board display and stale async completion, with visible loading feedback. Route: delegated writer (TUI implementation and tests). Checks: direct Update/View tests cover A→B overlap/errors, loading-time keys, post-refresh focus, and stale detail/transition results before/during/after board switching. Writer and final independent verifier: `go test ./internal/tui -count=1`, `go test ./...`, `git diff --check` passed. Parent spot-check `go test ./internal/tui -run 'TestBoard|TestLoading|TestStale' -count=1` passed. Live Jira/TTY unavailable. Rollback: revert this work unit in `internal/tui/tui.go` and `internal/tui/tui_test.go`. Commit and assessment evidence pending in T3-6.
- [ ] T3-2 — Migrate Bubble Tea, Bubbles, and Lip Gloss to v2; adapt main and setup models, key/window handling, rendering, and existing tests without changing their behavior. Route: delegated writer (multiple non-trivial files). Checks: focused TUI tests, full Go tests/build/vet and render tests. Commit and assessment evidence pending.
- [ ] T3-3 — Add ticket-detail modal and declarative help overlay on Kanban; update keyboard docs. Route: delegated writer (TUI/tests/README). Checks: overlay geometry, keyboard focus/back, existing render tests. Commit and assessment evidence pending.
- [ ] T3-4 — Add `/` live local filter and Enter server JQL search plus `s` direct server search; preserve sprint action under an unambiguous key and document it. Route: delegated writer (TUI/Jira/tests/README). Checks: filter behavior, escaped JQL, request races, result navigation, all Go tests. Commit and assessment evidence pending.
- [ ] T3-5 — Add contextual status and bounded async feedback (including transition activity); assess mouse wheel/animations as optional polish without destabilizing navigation. Route: delegated writer (TUI/tests/README). Checks: loading/error transitions, status rendering, focused/full Go tests. Commit and assessment evidence pending.
- [ ] T3-6 — Record work-unit identities and verification, close the checklist before the final freeze. Route: inline bookkeeping. Commit identities and final review declaration pending; no approval claim is pre-written.
- [x] T3-7 — Update `AGENTS.md` session-start backlog table to display explicit issue priority or `—` when absent. Route: inline single-file instruction change. Checks: re-read instruction and `git diff --check` passed; `gh issue list --state open --json number,labels` confirmed #3, #4 and #9 have no `priority:*` label. No inferred ranking or GitHub priority mutation. Rollback: restore only the Session Start priority wording in `AGENTS.md`. Commit identity pending in T3-6.

## Progress and evidence
- Initial `develop` worktree clean; local branch created through `dflow start feature tui-v2-modals --no-push`.
- Read-only video inspection and source mapping confirmed stale display and unscoped async results.
- T3-1 verified in `internal/tui/tui.go` and `internal/tui/tui_test.go`: stale cards/sprint hidden, scoped board/detail/transition completions, loading-time input guard, and focus restoration. Three independent passes exposed then resolved two race conditions; final pass found no blocker. `go test ./...` and `git diff --check` passed. Runtime Jira/TTY unavailable; commit and native assessment pending.
- T3-7 verified: `AGENTS.md` requires an explicit `priority:*` label or `—`. Open issues #3, #4, and #9 have no such label. GitHub Projects priority was inaccessible without `read:project`; do not invent a ranking. `git diff --check` passed.

## Next step
Mirror this record, then seek explicit staging/commit decision for closed T3-1 and T3-7 work units, listing T3-2 through T3-6 as open. Continue with T3-2 only after reviewing the committed candidates.
