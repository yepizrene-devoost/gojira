# Issue #11 — Review the unreviewed history as a chained sequence of small candidates

Status projection for the RDD review chain over `init..9e22bfe`.
Checkbox convention: `- [ ]` open, `- [x]` closed.

## Goal

Cover the unreviewed history (`9130f97..9e22bfe`, 24 commits, 40 files, 5454 changed
lines) with a chained sequence of review candidates that each fit under the provider's
context budget. The provider refuses the whole range as a single candidate
(`lens_context_budget_exceeded`), and its remedy is a chain of smaller candidates.

## Validated facts (2026-09-25, this session)

- The history is **linear**: `9130f97 → 3e0e822 → d883338 → 902fc77 → bde26bc → … →
  9c7a6fa → 9e22bfe`. Only `9e22bfe` is a merge, and `tree(9e22bfe) == tree(9c7a6fa)`,
  so the merge adds zero changed lines.
- `9130f97` ("init") is **not an app**: it holds a single 1-line `README.md`. Everything
  the project is lives inside `init..9e22bfe`, so the chain covers the whole codebase
  and there is no unreachable content behind the base.
- **Historical ranges are reviewable.** `gentle_review inspect` with `workspaceRoot`
  pointed at a detached worktree returned a valid START route: base `9130f97`,
  candidate tree `0393aad3…` (tree of `9e22bfe`), 40 changed paths, fresh lineage
  `review-1a1184486d9551e9`. The candidate is the workspace tree, so each slice needs a
  worktree checked out at that slice's top commit, with `baseRef` = previous boundary.
- The provider defaults `baseRef` to `init` in a worktree with no reviewed boundary
  (issue fact #1 confirmed); the reduced base has to be passed explicitly as
  `{"mode":"ordinary","baseRef":"<full 40-char SHA>","committedOnly":true}`.
- Worktrees live outside the repository at
  `/Users/rene/Documents/WorkspacesApps/.review-worktrees/gojira/<sha>` so they never
  appear as untracked paths in the main workspace candidate.

## Budget evidence

| Candidate | Size | Result |
|---|---|---|
| `9e22bfe → 80a313a` | 7 files, 173 changed lines | accepted, tier `high`, 4 lenses |
| `init..HEAD` at `80a313a` | 41 files, 5545 changed lines | refused at preflight, `not_started` |

The ceiling sits between those two and is still unprobed. A refusal is free
(`mutation_outcome: not_started`, no authority created), so probing downward from the
largest slice yields free information until the first admission.

## Slice plan (disjoint partition, chronological)

| # | Base | Top | Content (actual changed paths) | Changed lines |
|---|---|---|---|---|
| S1 | `9130f97` | `3e0e822` | monolithic app: `main.go`, `main_test.go`, `go.mod`/`go.sum`, `AGENTS.md`, `.dflow.yaml`, `.agents/workflows/dflow.md`, `.env.example`, `.gitignore`, `README.md` | **1342** |
| S2 | `3e0e822` | `902fc77` | `main.go`, `go.mod`/`go.sum` (clipboard JSON, transition + worklog) | 368 |
| S3 | `902fc77` | `bde26bc` | cobra CLI: `root.go`, `tui_cmd.go`, `boards_cmd.go`, `export_cmd.go`, `log_cmd.go`, `move_cmd.go`, `jira_client.go`, `jira_types.go`, `tui.go`, `main.go` | **2518** |
| S4 | `bde26bc` | `8b5fc18` | package split: `cmd/*`, `internal/{config,jira,tui}` | 258 git / **3120 provider** |
| S5 | `8b5fc18` | `912b446` | config, keychain, wizard (`cmd/config.go`, `internal/config`, `internal/tui/setup.go`) | 645 |
| S6 | `912b446` | `6313989` | read commands (`get`, `projects`, `search`, `export`) + F7.1 | 582 |
| S7 | `6313989` | `f056466` | write commands (`assign`, `comment`, `create`, `update`), `paths.go` | 516 |
| S8 | `f056466` | `8daa9da` | issue templates, Makefile, `cmd/version.go` | 328 |
| S9 | `8daa9da` | `51dab44` | `AGENTS.md`, `README.md` | 314 |
| S10 | `51dab44` | `246e865` | `cmd/create.go`, sprint membership, `paths.go` | 401 |
| S11 | `246e865` | `e658df9` | `LICENSE`, `assets/gojira-logo.png|svg` (image-heavy) | 617 |
| S12 | `e658df9` | `9b3b2d5` | `README.md` | 39 |
| S13 | `9b3b2d5` | `793ca82` | ADF fix (`adf_test.go`), pagination | 224 |
| S14 | `793ca82` | `9e22bfe` | accountId lookup via picker + JQL | 230 |

S1 (1342) and S3 (2518) are single oversized commits and **cannot be subdivided** —
candidate granularity is one commit. S11 is 617 "lines" but ~595 of them are image
assets, which are cheap in reviewer context.

## Tasks

- [x] S1 — review `9130f97..3e0e822` (probe: decided that the chain can reach the
      oversized commits — **abandoned with `operator_disposition`, see evidence log**;
      live finding recorded in issue #12)
- [x] S2 — review `3e0e822..902fc77` (**abandoned with `operator_disposition`** — 4
      CRITICAL findings, all already remediated on `develop` by `4f58abc`; no new
      issue needed, see evidence log)
- [x] S3 — review `902fc77..bde26bc` (**escalated, terminal** — see evidence log; 2
      live findings identified for recording, rest already remediated or historical)
- [x] S4 — review `bde26bc..8b5fc18` (**approved** — first approved receipt of the
      chain; lineage `review-dfdffb4de76736e1`, authority burned
      `gentle-ai.review-acknowledged/v1`; all 3 `follow_up` findings already tracked
      in #12/#13, see evidence log)
- [x] S5 — review `8b5fc18..912b446` (**escalated, terminal** — tier `medium`, single
      lens; `R3-003` unknown causality: missing unit tests for config/keychain code —
      a real, still-live gap recorded below; see evidence log)
- [x] S6 — review `912b446..6313989` (**escalated, terminal** — `R3-001` unknown
      causality; 2 live defects identified: naive JQL escaping in `SearchJQL` and
      unguarded `Created[:10]` slice; see evidence log)
- [x] S7 — review `6313989..f056466` (**abandoned with `operator_disposition`** —
      severe findings all covered by existing issues or obsoleted by later fixes; no
      new issue needed, see evidence log)
- [x] S8 — review `f056466..8daa9da` (**escalated, terminal** — the escalated BLOCKER
      was refuted by an actual build; 2 live minors recorded below, see evidence log)
- [x] S9 — review `8daa9da..51dab44` (**approved** — clean pass, no findings;
      lineage `review-cffb453147fa46c4`, tier `medium`, authority burned
      `gentle-ai.review-acknowledged/v1`)
- [x] S10 — review `51dab44..246e865` (**approved** — third approved receipt; 9
      advisory findings, all informational, none tracked as live defects; lineage
      `review-5057291d36be89e5`, authority burned)
- [x] S11 — review `246e865..e658df9` (**approved** — tier `medium`, 1 lens, no
      findings; lineage `review-aa45c1e4267629ae`, authority burned)
- [x] S12 — review `e658df9..9b3b2d5` (**approved at START** — tier `low`,
      `non_executable_only` README change, zero lenses; lineage `review-333a3bb98b1b3a86`)
- [x] S13 — review `9b3b2d5..793ca82` (**approved** — ADF fix + pagination; 3
      advisory findings, informational; 2 schema-defect refusals, third attempt
      admitted; lineage `review-c2f829fecdfbe103`, authority burned)
- [x] S14 — review `793ca82..9e22bfe` (**approved** — accountId via picker + JQL; 2
      advisory findings, informational; lineage `review-d5e1caa41930c864`, authority
      burned)
- [ ] Consolidate findings into issues (fold into #10 where they belong) and close #11

## Open decisions (human)

- How far the chain goes. Each admitted slice is a full 4-lens lifecycle; a 14-slice
  chain is ~14 lifecycles. Cost is explicitly a user decision (issue #11).
- If S3 is refused, `bde26bc` (46% of the debt, the whole CLI surface) is unreachable
  at commit granularity and has to be declared legacy.

## Evidence log

- 2026-09-25: mechanism validated read-only at `9e22bfe` and at `3e0e822`; worktrees
  created for `3e0e822` (S1 probe) and `bde26bc` (S3 probe); boundary diffs measured
  with `git diff --shortstat` and `--name-only`.
- 2026-09-25: **S1 probe ACCEPTED.** Lineage `review-ede88802712caa9d`, tier `high`
  (reason `process_boundary`/`shell_process` in `main.go`), 10 files, 1342 changed
  lines, 4 lenses (`risk`, `resilience`, `readability`, `reliability`), correction
  budget 200. **The ceiling is therefore >= 1342 lines / 10 files, and a single
  oversized commit does fit.** Forecast relayed: 4 reviewer model runs; acknowledged
  by the user (S1 only).
- 2026-09-25: **S1 reviewers ran and were admitted** (4 host-relay runs, ~51.5 KB
  prompt each); the provider then required one refuter run, which confirmed three
  CRITICAL candidate-caused findings in the monolithic `main.go` and moved the lineage
  to `correction_required` (finding IDs `R3-json-unmarshal-errors`, `R4-001`,
  `R4-002`).
- 2026-09-25: **Human decision — do not correct the historical snapshot.** Verified
  against the current tree: `R3`/`R4-002` (unchecked `json.Unmarshal`) are already
  remediated on `develop` by `4f58abc`; `R4-001` (discarded `io.ReadAll` error) is
  still live at `client.go:42,59,355,379` and belongs in the current tree, not in the
  snapshot. The user chose to record and continue: issue **#12** created for `R4-001`
  (`type:bug`, `priority:low`), and lineage `review-ede88802712caa9d` abandoned with
  reason `operator_disposition` (native `review-reclaim-record/v1`, status
  `committed`, actor `rene`, expected revision `sha256:ac4143b8…81e379`).
- 2026-09-25: **S2 review ran; same disposition.** Lineage `review-e9436ca25256d405`,
  tier `high`, 3 files (`go.mod`, `go.sum`, `main.go`), 368 lines, correction budget
  184. 4 CRITICAL deterministic findings (`R3-unmarshal-gettransitions`,
  `R3-unmarshal-getworklog`, plus duplicates `R4-001`/`R4-002` from the resilience
  lens): unchecked `json.Unmarshal` in `GetTransitions` (main.go:306/307) and
  `GetWorklog` (main.go:320/325). Verified against the current tree: both methods in
  `internal/jira/client.go` check the unmarshal error (fix `4f58abc`), so **all four
  findings are already remediated on `develop`**. No new issue. Lineage abandoned with
  reason `operator_disposition` (record `committed`, actor `rene`, expected revision
  `sha256:78ee5c84…eba369`).
- 2026-09-25: **Provider relay defect observed (S2)**: the reviewer relay twice
  produced a result whose `findings[]` entries carried an extra `evidence` field,
  which the reviewer/v1 schema rejects (`additionalProperties: false`; `evidence` is
  only valid at top level as a string array). Refusals:
  `json: unknown field "evidence"`, payloads preserved under
  `.git/gentle-ai/rejected-results/review-e9436ca25256d405/`. The third attempt
  complied and was admitted. Refused slots are never consumed and were retried only
  via a fresh STATUS-offered binding, never by resubmitting refused bytes.
- 2026-09-25: **S3 live findings recorded in issue #13** (`harden(jira): add HTTP
  retries and escape issue keys in REST paths`, `type:bug`, `priority:low`), per the
  user's decision to use one combined issue. Remaining S3 findings need no action
  (see disposition above).
- 2026-09-25: **S3 closed as `escalated` (terminal `native_stop_required`).** After a
  bounded retry (user-approved, max 2; succeeded on the first), `review-reliability`
  was admitted and all 4 lenses completed. The refuter corroborated `R4-001` and left
  `R3-003` with unknown causality, so the provider escalated instead of attributing
  it. Findings disposition against the current tree: `R3-003` (module path change in
  `go.mod`) is an intentional historical change, no action; `R4-001` (no HTTP
  retries/backoff in the client) is **live**; `R1-001` (issue keys interpolated into
  REST paths without `url.PathEscape`) is **live**, low-severity hardening;
  `R3-001`/`R4-002`/`R2-silent-unmarshal` (unchecked `json.Unmarshal`) were already
  remediated by `4f58abc`; `R3-005` (ignored `io.ReadAll` error) is issue #12; the
  remaining WARNING/SUGGESTION findings are historical-monolith observations recorded
  here for reference (bounds check in the old kanban scroll, silent sprint fallback
  in `export --sprint`, undocumented exact transition-name matching, `minInt`
  shadowing the Go 1.21 builtin, dead `domain` field in the old TUI model). Total
  relay cost for S3: 4 lens runs + 2 refused reliability attempts + 1 bounded retry +
  1 refuter.
- 2026-09-25: **Provider does not use git rename detection.** S4's slice is a pure
  package split (git: 12 files, 258 lines with rename collapse), but the provider
  froze it as **22 files / 3120 changed lines** (every moved root file counted as a
  full deletion plus a full addition). Still accepted — the ceiling holds above 3120
  lines — but rename-heavy slices cost far more than `git diff --shortstat` suggests.
  Slice table now shows both numbers for S4.
- 2026-09-25: **S4 APPROVED.** Lineage `review-dfdffb4de76736e1` (tier `high`, 22
  files, 3120 provider-counted lines, correction budget 200) reached `approved` with
  no refuter and no correction — 4/4 lens runs admitted, zero retries needed. 11
  advisory findings, none blocking: `R3-3-1`/`R3-3-2` (ignored `io.ReadAll` in
  `get`/`post`) map to issue #12; `R4-001` (no HTTP retries) maps to issue #13; the
  rest are informational (stale `buildClient` doc comment, misleading `tui --help`
  default, dead config skeleton code, browser-open risk note). Authority burned via
  `acknowledge-approved` (`gentle-ai.review-acknowledged/v1`, consumed revision
  `sha256:8e48f314…ba369`→`sha256:8e48f314eb0786ce781592f2c94f1232be0b5d6ea667661f049b9f3c35a982fc`,
  delivery left to ordinary repository policy).
- 2026-09-25: **S5 closed as `escalated` (tier `medium`, lens `review-reliability`
  only, 7 files, 645 lines, lineage `review-a957d0a6dac0dd11`).** Findings:
  `R3-001` (WARNING, ignored `reader.ReadString` error in the wizard prompt loop,
  `cmd/config.go:39`), `R3-002` (WARNING, `SaveToken` ignores the config-save error
  after a successful keyring write, `internal/config/config.go:102`), `R3-003`
  (CRITICAL, no unit tests for config save/load and token storage) — the last one
  escalated with unknown causality because an absence cannot be attributed to
  candidate or base. All three remain live in the current tree (`internal/config`
  still has no tests); candidate follow-up issue, not yet filed.
- 2026-09-25: **S5 live findings recorded in issue #14**
  (`test(config): cover config/token storage and surface wizard/save errors`,
  `type:bug`, `priority:low`), per the user's decision.
- 2026-09-25: **S6 closed as `escalated` (tier `high`, 4 lenses + refuter, 8 files,
  582 lines, lineage `review-bcd56701e409ae79`).** The refuter corroborated `R3-002`
  (naive `strings.ReplaceAll(jql, "'", "\\'")` escaping in `SearchJQL`) and left
  `R3-001` with unknown causality (unverified location), forcing escalation. Verified
  against the current tree: **live** — `SearchJQL` still uses the naive escape
  (`client.go:205`; the `url.QueryEscape` at line 469 belongs to `ResolveAccountID`),
  and `cmd/get.go:115` still slices `c.Created[:10]` unguarded (panic on short
  timestamps). Already fixed elsewhere: unchecked `json.Unmarshal` (`4f58abc`), issue
  keys in paths (#13), ADF flatten has tests (`adf_test.go`); remaining findings are
  test-coverage gaps for `cmd/get|projects|search` and code-quality warnings (duplicated
  ADF flatten/truncate, dead `TimeLogged` field, brittle date slicing).
- 2026-09-25: **S6 live defects recorded in issue #15**
  (`fix(jira): properly URL-encode JQL in SearchJQL and guard Created[:10] in get`,
  `type:bug`, `priority:medium`), per the user's decision.
- 2026-09-25: **S7 review ran; abandoned with `operator_disposition`.** Lineage
  `review-e5ccdc6a806d16cd` (tier `high`, 4 lenses, 10 files, 516 lines, write
  commands Fase 8 + `paths.go`), `correction_required` with deterministic findings,
  no refuter. Disposition against the current tree: ResolveAccountID findings
  (injection via `username=` query, hardcoded `/user/username` path, empty-response
  ambiguity) are **obsolete** — that endpoint and method were rewritten by `9c7a6fa`
  after Atlassian removed the API; heading `text:"2"` attrs and discarded mention
  accountId were **fixed by `d64f7a7`**; unchecked `json.Unmarshal` by `4f58abc`;
  issue-key escaping in write paths and missing retries map to issue **#13**.
  Remaining untracked items are minor quality notes, recorded here: fragile
  code-block state in `TextToADF` (WARNING, rewritten parser may still share the
  pattern), `update --labels` replaces instead of appending, duplicated PUT block in
  `AssignIssue`/`UpdateIssue` (still no shared `put` helper), asymmetric board guard
  (`ProjectKey` test vs `ProjectName` print), `comment --mention` positional-arg
  mismatch, unused `fieldsExport` constant, setup error stored as string.
- 2026-09-25: **S8 closed as `escalated` (tier `high`, 4 lenses, 9 files, 328 lines,
  lineage `review-051cff6b55f73ed9`).** The escalated finding was a BLOCKER claiming
  `8daa9da` removed a `return m, nil` and did not compile; the refuter left it
  `unverified_location` and the provider escalated rather than admit an uncorroborated
  blocker. Verified by running `go build ./...` at the snapshot worktree: **exit 0,
  the claim is false.** Live minors recorded here (no issue, trivial): `var version =
  "dev"` declared in both `main.go:14` and `cmd/version.go:13`, and discarded
  `json.MarshalIndent` error in `cmd/version.go:48`.
- 2026-09-25: **S11 APPROVED** (lineage `review-aa45c1e4267629ae`, tier `medium`, 1
  lens, LICENSE + logo assets, no findings; authority burned).
- 2026-09-25: **S12 APPROVED at START** (lineage `review-333a3bb98b1b3a86`, tier
  `low`, `non_executable_only` README-only change, zero lenses, no reviewer runs).
- 2026-09-25: **S13 APPROVED** (lineage `review-c2f829fecdfbe103`, tier `medium`, ADF
  fix + pagination, 3 advisory findings informational). Two refusals with the
  recurring `evidence`-field relay defect before the third attempt was admitted —
  both rejected payloads had the identical filename hash, suggesting deterministic
  model output for that prompt.
- 2026-09-25: **S14 APPROVED, chain complete** (lineage `review-d5e1caa41930c864`,
  tier `medium`, accountId lookup, 2 advisory findings informational, authority
  burned). All 14 slices processed; 5 approved receipts (S4, S9, S10, S11, S13, S14 =
  6 including S12's zero-lens approval), 2 abandoned (S1, S2, S7 = 3), 4 escalated
  (S3, S5, S6, S8). Live defects filed: #12, #13, #14, #15.
- 2026-09-25: **S9 APPROVED.** Lineage `review-cffb453147fa46c4` (tier `medium`,
  single `review-reliability` lens, 2 files — `AGENTS.md`/`README.md`, 314 lines,
  correction budget 157) reached `approved` with zero findings; authority burned via
  `acknowledge-approved` (consumed revision `sha256:8fcf40c6…02279`→`sha256:8fcf40c6053dc6bf6714ad333e3acad19a51aba783a75c8d70a2d62982b60279`).
  Second approved receipt of the chain.
- 2026-09-25: **S10 APPROVED.** Lineage `review-5057291d36be89e5` (tier `high`, 4
  lenses, 4 files, 401 lines, TUI create + sprint membership) reached `approved` with
  9 advisory findings (all informational: active-state literal, duplicated
  reload/focus, nil-guard condition, sentinel zero board issues, task default
  duplication, and reliability warnings on `client.go:243-285` / `cmd/create.go`).
  One readability reviewer attempt failed transiently (`524` upstream, slot not
  consumed) and was retried successfully. Authority burned via
  `acknowledge-approved` (consumed revision `sha256:0c71760d…60e737`).
- 2026-09-25: **S3 probe ACCEPTED** — lineage `review-a6d6cdbdaba301c6`, tier `high`,
  12 files, 2518 changed lines, 4 lenses, correction budget 200. The largest single
  commit of the whole debt fits, so the chain can cover 100% of `init..9e22bfe`.
- 2026-09-25: **S3 review in progress (3 of 4 lenses admitted).** The first group
  attempt aborted before any submission: the `review-resilience` reviewer hit its
  output limit (`stopReason: length`) and produced no text; no slots were consumed.
  Switched to per-slot capture: `review-risk` (prompt 83.6 KB), `review-resilience`
  and `review-readability` were captured and admitted. `review-reliability` was
  refused twice at admission with the same relay schema defect
  (`json: unknown field "evidence"`, payloads under
  `.git/gentle-ai/rejected-results/review-a6d6cdbdaba301c6/`); the slot is still open
  and awaits a human decision on further retries.
