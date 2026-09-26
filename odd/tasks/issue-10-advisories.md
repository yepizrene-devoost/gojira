# Issue #10 — Address the 5 advisory findings from the first RDD review

Status projection for the advisory cleanup work unit.
Checkbox convention: `- [ ]` open, `- [x]` closed.

## Goal

Close the 5 non-blocking advisory findings (`disposition: informational`) emitted by the
first RDD review, whose lineage is consumed (`review-e5756e447a55658a`, verdict
`approved`, authority burned). The native closure is explicit that these are not a reason
to reopen that candidate: implementing them mints a **new** review candidate.

Branch: `feature/issue-10-advisories` (from `develop`, dflow, `--no-push`).

## Findings and disposition

| # | Finding | Lens / severity | Location | Disposition |
|---|---|---|---|---|
| 1 | `R2-cliff-parser-fallback` | readability, SUGGESTION | `cliff.toml:29-39` | fix: explicit catch-all group |
| 2 | `R2-comment-truncation-magic` | readability, SUGGESTION | `cmd/get.go:111-112` | fix: named constant + rune-safe truncation |
| 3 | `R2-inconsistent-unmarshal-swallow` | readability, **WARNING** | `internal/jira/client.go:483-485` | fix: propagate with context |
| 4 | `R3-001` | reliability, **WARNING** | `internal/jira/client_test.go` | fix: table case per decode path |
| 5 | `R3-002` | reliability, SUGGESTION | `cmd/get.go:91` | fix: helper owning the discard |

## Validated facts (2026-09-26)

- **Finding 1 is partly wrong in the current toolchain.** The issue says unlisted
  conventional types "land outside every named group". Verified against git-cliff
  **2.14.2** on a scratch repo: `build:`, `ci:` and `perf:` commits are *not* dropped —
  git-cliff auto-derives a group named after the raw lowercase type (`### build`,
  `### ci`, `### perf`). `filter_unconventional = true` still drops non-conventional
  messages. So the real defect is not data loss but **unstated, uncapitalized groups**
  that appear only if such a commit ever lands. The fix (explicit catch-all `Other`) is
  the one the finding asks for and makes the behavior stated instead of implied.
- Only one git-cliff behavior was assumed and it was tested: with the catch-all added,
  the same scratch repo renders `## Other` for `build`/`ci`/`perf`, `## Chores` still
  catches `chore!:` (breaking-change marker), and the unconventional commit stays
  filtered (1 skipped).
- `resolveViaJQL` is called from exactly one place (`ResolveAccountID`), so its
  signature can change without touching callers elsewhere. Its `ok bool` cannot
  distinguish "no match" from "request/decode failed".
- The 14 sibling decode paths changed in `4f58abc` return the raw `json.Unmarshal` error
  without wrapping; context is added by the *caller's* error message. The fix for
  finding 3 follows that convention instead of inventing a wrapping style.
- `cmd` has no test file today (tests live in `internal/jira`, `internal/tui`); the
  rune-truncation helper is pure, so a `cmd/get_test.go` needs no network or TTY.
- The 6 identical `_, _ = fmt.Fprintf` discards in `internal/tui/tui.go` are **out of
  scope**: finding 5 cites `cmd/get.go:91` only, and that file carries its own review
  history. Left as noted follow-up, not silently changed.

## Design decisions

1. **Catch-all group name**: `Other`, capitalized like the named groups, appended last
   so every existing parser keeps priority. Not per-type `Build`/`CI`/`Performance`
   groups: `AGENTS.md` documents exactly 7 conventional types, so a type outside them is
   by definition "other", and one rule covers future types too.
2. **Truncation**: `commentPreviewMax = 200` counts **runes**, ellipsis included
   (197 runes + `...`), so an accented Spanish comment can no longer split a rune
   mid-sequence. Behavior for ASCII input is byte-identical to today.
3. **`resolveViaJQL`**: returns `(*UserRef, error)` — `(nil, nil)` means "no match", a
   non-nil error means the fallback could not be evaluated. `ResolveAccountID` reports
   picker and JQL failures separately instead of collapsing a transport/decode failure
   into "no such user". Error text for the picker-only failure path is unchanged.
4. **Discarded builder writes**: one local `writef` helper in `cmd/get.go` documents why
   the error is uninteresting (`strings.Builder.Write` never fails) instead of spelling
   `_, _ =` at each site.

## Tasks

- [x] T1 — `chore(changelog)`: explicit catch-all group in `cliff.toml`; verified with
      `make changelog` on a scratch repo.
- [x] T2 — `fix(cli)`: rune-safe comment preview with a named constant + `cmd/get_test.go`.
- [x] T3 — `refactor(jira)`: `resolveViaJQL` propagates its failure; `ResolveAccountID`
      reports picker and JQL errors separately.
- [ ] T4 — `test(jira)`: malformed 2xx body surfaces an error on every decode path.
- [ ] T5 — `refactor(cli)`: route the discarded builder writes through `writef`.

## Verification

- `make check` (`go vet ./...` + `go test ./...`) must pass after every task.
- `make changelog` must render the new group without dropping existing ones.
- No behavior change is intended for the happy paths; the only observable change is a
  richer error message on a failed JQL fallback.

## Evidence log

| Task | Commit | Verification |
|---|---|---|
| T1 | _pending_ | scratch render shows `## Other` for `build`/`ci`/`perf`, `## Chores` keeps `chore!:`; real-history render byte-identical to the old config (4 filtered commits in both) |
| T2 | _pending_ | `go test ./cmd/` — 5 helper cases + the renderer wiring case pass; `gofmt -l cmd/get.go cmd/get_test.go` clean |
| T3 | _pending_ | `go test ./internal/jira/` — all 6 `TestResolveAccountID` cases pass unchanged; error text is now `(picker: …; jql: …)` with only the failing sources listed |
| T4 | _pending_ | `go test ./internal/jira/` |
| T5 | _pending_ | `make check` |

Commit identities are recorded as each task closes; the final `docs(odd)` commit
closes the commit-identity stage.

## Open follow-ups

- `internal/tui/tui.go:1072-1086` repeats the same 6 discarded `fmt.Fprintf` writes; not
  covered by finding 5 and deliberately left untouched.
- `R3-001` grew into real work (a decode-error contract sweep over every decode path):
  per the issue, promote it to its own tracker only if it turns into more than this pass.
- Repo-wide `gofmt` drift, **pre-existing in `develop`** and unrelated to these findings:
  `gofmt -l` flags `cmd/tui.go`, `internal/jira/client.go` (a `CreateIssue` map
  alignment), `internal/jira/paths.go`, `internal/jira/types.go`, `internal/tui/tui.go`
  (an import-order swap). There is no CI or lint gate that would catch it — `.github/`
  holds issue templates only, and `make lint` needs a locally installed
  `golangci-lint`. Left out of this candidate on purpose: fixing one file here would
  split one repo-wide condition across two commits and add unrelated diff noise.
  Proposed home: issue #8 (release pipeline / CI).
