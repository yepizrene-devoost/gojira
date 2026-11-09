# Self-upgrade: `gojira upgrade` and TUI update notification

Implements GitHub issue #41 in one delivery: a dedicated `gojira upgrade` command that
replaces an installer-owned binary with the latest published release, plus a
non-blocking TUI notification for a newer release. `gojira update` keeps its Jira
contract untouched.

Branch: `feature/issue-41-self-upgrade`. No release, merge or issue closure is implied.

## Specs

S1 — Version comparison. `gojira upgrade` compares the injected build marker
(`cmd/version.go:15`, `-X main.version`) with the latest published release tag. Only
`vMAJOR.MINOR.PATCH` tags are comparable. A local marker that is not a release version
(`dev`, dirty checkout, snapshot) is never replaced: the command fails with
`<path> is a development build (version "<marker>"); install a published release with scripts/install.sh or scripts/install.ps1`.
`--check` still reports the latest release for such builds.

S2 — Release metadata. The latest release comes from the GitHub REST API
`/repos/yepizrene-devoost/gojira/releases/latest` over HTTPS with its own
`http.Client` (no Jira client reuse, no credentials, no `Accept`-based auth). The
request has a bounded timeout, sends no user data beyond the plain GET, and any
failure is reported as
`release check failed: <cause>; nothing was changed` with mutation state
`not_applied`. Draft and prerelease tags are never offered. Offline use leaves every
local file untouched.

S3 — Artifact and checksums. The selected asset follows `.goreleaser.yaml:22-32`:
`gojira_<Os>_<arch>` where `Os` is title-cased and `amd64` renders as `x86_64`, with
`.tar.gz` except Windows `.zip`. `checksums.txt` from the same release must contain
exactly one entry for that asset whose digest is 64 hex characters; a missing, empty,
duplicated or malformed entry aborts before any download is written. The command states
that a matching checksum proves transfer integrity only: it does not authenticate the
publisher, because the file is served next to the artifact.

S4 — Download and extraction safety. The archive is streamed to a temporary file in the
destination directory with a size cap, never fully buffered, and never in `/tmp`. Only
one archive entry named `gojira` (or `gojira.exe` on Windows) at the archive root is
extracted; absolute paths, `..` traversal, symbolic links, directories and extra `gojira`
entries abort the upgrade. Metadata files shipped inside the archive (README, LICENSE,
CHANGELOG) are ignored.

S5 — Ownership. Replacement happens only when the resolved executable is a regular file
whose directory is `$GOJIRA_INSTALL_DIR`, `~/.local/bin`, or on Windows
`%LOCALAPPDATA%\Programs\gojira`. Every other case — package-manager paths, `go install`
(GOPATH/bin), symlinks, relative paths, directories, other users' directories — is
refused before writing anything, with
`refusing to replace <path>: not a gojira installer-owned location (set GOJIRA_INSTALL_DIR or run the installer)`.
The destination directory itself must not be a symbolic link — matching
`scripts/install.sh` — which is refused with
`refusing to replace <path>: <dir> is a symbolic link`, and
`%LOCALAPPDATA%\Programs\gojira` is an owned directory only when `GOOS` is `windows`.
Neither installer writes a receipt (`scripts/install.sh`, `scripts/install.ps1`), so the
directory layout is the only ownership evidence.

S6 — Noninteractive contract. Without a terminal and without `--yes`, the command fails
before any network mutation with
`refusing to install without confirmation; rerun with --yes or use --check`
and mutation state `not_applied`. `--check` never installs and needs no confirmation.

S7 — Replacement. The verified binary is staged as a mode-0755 temporary file in the
destination directory, flushed, then moved into place atomically (`os.Rename`). On
Windows the current binary is first renamed to `gojira.exe.old.<suffix>`, the staged
file is moved to the destination, and the leftover is removed when possible; when the
running image cannot be deleted, the command reports the leftover path and its removal
is deferred to a later upgrade. The previous binary is preserved under a `.old.<suffix>`
name next to the destination, so it is never lost; a both-steps-failed placement and a
process-kill window are reported as `unknown`, never silently. An exclusive lock file in
the destination directory prevents concurrent upgrades and yields
`another upgrade is already running for <dir> (remove <lockFilePath> if no upgrade is running)`.

S8 — Outcome reporting. Report `applied`, `not_applied` or `unknown` per the existing
contract (`internal/jira/client.go:108-116`, `cmd/json.go:16-33`), reusing the same JSON
field `mutationState`. A staged-but-not-placed binary is `unknown`; a failed download,
checksum mismatch or refused ownership is `not_applied`; a placed binary is `applied`.
No automatic retry follows an `unknown` outcome.

S9 — CLI surface. New command `gojira upgrade` with `--check`, `--json`, `--yes` and
`cobra.NoArgs`, registered with the existing per-file `init()` pattern. Human output:
`✓ Upgraded gojira <old> → <new> (restart any running session)`,
`✓ gojira <current> is the latest published release`, and
`gojira <current> → <new> available; run gojira upgrade to install it`.
`--json` writes one `schemaVersion: "v1"` document on stdout with `currentVersion`,
`latestVersion`, `updateAvailable`, `action` (`installed|available|up_to_date|unknown|declined`),
`mutationState`, `executablePath`, `assetName`, `checksumVerified`; diagnostics stay on
stderr through the existing error envelope. `--check` exits 0 whether or not an update
exists.

S10 — TUI notification. When `update_check` is enabled and the recorded check is older
than 24 hours, the TUI issues one asynchronous `tea.Cmd` release check with a bounded
timeout on start. A newer release produces exactly one non-blocking line per session,
`GoJira <new> is available — run gojira upgrade`, rendered as text (not color-only),
dismissible like an existing toast. Failures, offline use and `dev` builds show nothing.
The TUI never installs anything.

S11 — Configuration. `internal/config` gains `update_check` (bool, default `true`),
`update_last_check` (RFC3339) and `update_latest_version` (string). Only `update_check`
is user-settable through the closed key switch in `cmd/config.go:23-95`; the two cache fields
are written by the binary, never by the human, and the onboarding wizard is unchanged. No secrets are stored. Help
text, `README.md` and the config documentation describe the setting, the cadence and how
to disable it.

S12 — Preserved contracts. `gojira update <issue-key>` arguments, flags, output and exit
codes are unchanged. Valid JSON stays on stdout and diagnostics on stderr. Jira endpoints
remain centralized in `internal/jira/paths.go`; the single TUI model stays a single
Bubble Tea model with string-state switching. No MCP surface is added for `upgrade`.

S13 — Verification. Strict test-first: each rule above gets one deterministic RED test
before its implementation. Tests use an `httptest` fake release server, temporary
directories and a fake executable path; the user's real binary is never replaced. Runs
cover release resolution, asset selection, checksum success and each rejection, archive
traversal and link rejection, ownership refusals, noninteractive refusal, concurrent
lock, staged replacement and Windows rename sequence, unchanged `gojira update`, and the
TUI notification gate. Full evidence: `go test ./...`, `go vet ./...`,
`golangci-lint run`, plus a manual `--check` against the real release after approval.

## Tasks

- [x] **T1** Release metadata client and version comparison in `internal/upgrade` — S1, S2, S13 — route: inline — risk: none — commit: `c527e8e` (479 lines)
- [x] **T2** Artifact selection, checksum verification, bounded download and safe extraction — S3, S4, S13 — route: inline — risk: none — commit: `b651130` (765 lines)
- [x] **T3** Ownership detection, lock, staged atomic replacement and outcome states — S5, S7, S8, S13 — route: inline — risk: item 5 (replaces an installed executable) — commit: `2a877cc` (715 lines)
- [x] **T4** `cmd/upgrade.go` wiring: `--check`/`--json`/`--yes`, help, README — S6, S9, S12, S13 — route: inline — risk: item 5 — commit: `bb7ab9e` (738 lines)
- [x] **T5** Config keys and TUI notification — S10, S11, S13 — route: worker (config/TUI wiring) + worker (diff de-churn) — risk: none — commit: `c85580f` (446 lines)
- [x] **T6** Full verification and evidence record — S13 — route: inline + independent verify — risk: none — commit: this bookkeeping commit

## Log

L1 (request): "integraremos el issue 41" — GitHub issue #41 "[Feature]: add safe GoJira
self-upgrade and update notification" (`status:approved`), whose body states: "Design and
implement a dedicated `gojira upgrade` command to check for and install a newer published
GoJira release, preserving `gojira update` and its existing behavior." and "Do not update
silently; decide opt-in, check cadence, offline behavior and accessibility during design."
and "Do not imply that checksums hosted alongside artifacts independently authenticate the
publisher." and "No implementation, release, merge or issue closure is authorized by this
issue."

L2 (decisions, answered verbatim through the harness questionnaire): scope "CLI +
notificación TUI"; ownership "Solo layouts del instalador (recomendado)"; noninteractive
"Requiere `--yes` (recomendado)"; Windows "Staging + rename y reinicio (recomendado)".

L3 (repository facts, verified by read-only exploration): version marker
`main.version` (`Makefile:15`, `.goreleaser.yaml:18-19`, `cmd/version.go:15`); `update`
is a Jira command (`cmd/update.go:13,27`) and must not change; no GitHub API usage and no
self-update dependency exists; installers resolve the latest tag by redirect and write no
ownership marker (`scripts/install.sh:126-146`, `install.ps1:135-143`); assets and
`checksums.txt` per `.goreleaser.yaml:22-39`; archive includes README, LICENSE and
CHANGELOG; mutation states live in `internal/jira/client.go:108-116`; TUI entry points
`internal/tui/tui.go:487,508` with toast infrastructure at `:267,:3282` and `WithVersion`
at `:3739`.

L4 (implementation evidence, T1-T5). Every rule was written as a failing test first and
observed failing before its implementation:
- T1: `go test ./internal/upgrade/` failed with `undefined: NewClient`, `undefined:
  DefaultReleasesURL`; GREEN after `version.go` and `release.go`. One test defect was
  corrected during RED: the pinned path cannot be observed through a complete fake base URL,
  so `NewClient` now takes the API base and appends the pinned `latestReleasePath`.
- T2: `go test ./internal/upgrade/` failed with `undefined: ArtifactName`, `undefined:
  NewFetcher`; GREEN after `artifact.go` and `fetch.go`.
- T3: `go test ./internal/upgrade/` failed with `undefined: System`, `undefined: NewInstaller`,
  `undefined: ErrNotOwned`; three expectation defects in the harness were corrected (rename
  recording format, over-broad failure injection) and the implementation followed; GREEN.
- T4: `go test ./cmd/ -run TestUpgrade` failed on undefined seams; one test expectation was
  corrected because the confirmation prompt must go to stderr to keep stdout machine-readable.
- T5: `go test ./internal/config/ ./cmd/ ./internal/tui/` failed to build (`UpdateCheck`,
  `UpdateCheckEnabled`, `UpdateCheckDue`, `UpdateCheckInterval`, `RecordUpdateCheck`,
  `WithUpdateCheck`, `updateAvailableMsg` all undefined) before the config and TUI wiring.
  A first worker pass reached GREEN but reformatted `internal/tui/tui.go` from a
  non-gofmt-clean base, producing a 603-line diff; that was reverted to HEAD and the feature
  re-applied for a 49-line diff, with equivalence to the reverted file proven by
  `gofmt internal/tui/tui.go | diff - /tmp/tui_with_feature.go`.

Suite and static checks: `go test ./...` all packages ok; `go vet ./...` clean;
`golangci-lint run` → 0 issues (one `errcheck` finding in the new `internal/upgrade/fetch_test.go`
was fixed). `gofmt -l cmd internal` lists only `internal/tui/tui.go` and
`internal/tui/tui_test.go`, which are unformatted at HEAD too; every file this work adds or
edits is gofmt-clean.

Shared-code differential (single script, base commit built into a scratch directory, never
committed as a test): `config set/get` for `domain`, `email`, `board`, `project` and
`people.<alias>` produced byte-identical stdout, stderr, exit codes and `config.yaml` on the
base and on this change. The intended differences are exactly three: the base refuses
`update_check` with `unknown key "update_check"` while this change accepts it, `config get
update_check` prints `false` after being disabled, and both unknown-key messages now list
`update_check` among the valid keys.

Real-network read-only check with a locally built binary: `gojira upgrade --check` printed
`gojira dev is not a published release; the latest published release is v0.5.0` (exit 0) and
`--check --json` emitted the v1 document with `action: unknown`, `updateAvailable: false`,
`mutationState: not_applied`; `gojira upgrade` without a terminal refused with
`refusing to replace /tmp/gojira-new: not a gojira installer-owned location (set
GOJIRA_INSTALL_DIR or run the installer)` and exit 1. No binary on the machine was replaced.

L5 (independent verification, T6). A separate read-only verifier re-read the implementation
and tests against every spec, ran `go test ./...`, `go vet ./...` and `golangci-lint run`, and
returned PASS for S1-S12 with zero blocking defects. Its follow-ups, all closed here with
one RED-first test per behavior change:

- S7 lock message was not actionable → the error now names the lock file:
  `another upgrade is already running for <dir> (remove <lockFilePath> if no upgrade is running)`,
  still wrapped so `ErrLocked` keeps matching. RED observed, then GREEN.
- S5 a symlinked destination directory could alias an unowned location → the directory is now
  `Lstat`-checked and refused (`refusing to replace <path>: <dir> is a symbolic link`), which
  matches what `scripts/install.sh` already refuses. RED observed, then GREEN.
- S5 `%LOCALAPPDATA%\Programs\gojira` was honoured on every platform → ownership of that
  directory is now gated on `GOOS == "windows"`. RED observed (`Target error = <nil>, want
  ErrNotOwned`), then GREEN.
- S7 stale `.gojira.new.*` staged files accumulated beside the destination → `CleanLeftovers`
  now removes both suffixes without ever touching the destination. RED observed, then GREEN.
- The Windows claim "the destination is never left missing" was broader than the code: the
  previous binary is moved aside so it is never lost, a failed placement is rolled back, and a
  window where both steps fail (or the process is killed between them) is reported as `unknown`
  with the leftover path and a verify-before-retrying instruction. SPEC S5/S7 and README now
  state exactly that. No test: documentation only.

Final evidence after the follow-ups: `go test ./...` all packages ok, `go vet ./...` clean,
`golangci-lint run` → 0 issues, `gofmt -l internal/upgrade cmd` clean (the only unformatted
files in the repository are `internal/tui/tui.go` and `internal/tui/tui_test.go`, unformatted
at HEAD as well).

Verification limits, stated honestly: the verifier could not re-derive RED-before-GREEN
ordering from the artifacts, because the work is uncommitted and every file carries the same
hour; the ordering evidence for T1-T5 is L4 above, and for the follow-ups it is the observed
failures quoted in this session. Windows replacement was exercised through the `System` seam on
this platform, never on a real Windows host.

L6 (commit boundaries). The user authorized the commit set and declined RDD. Five work-unit
commits on `feature/issue-41-self-upgrade`, each compiling and passing its own focused tests
before the next one, then this `docs(odd)` bookkeeping commit:

1. `c527e8e` feat(upgrade): compare published releases and resolve the latest one — 479 lines,
   `internal/upgrade` version and release metadata with their tests.
2. `b651130` feat(upgrade): verify release artifacts and extract them safely — 765 lines,
   artifact naming, checksums, bounded download and safe extraction with their tests.
3. `2a877cc` feat(upgrade): replace only an installer-owned binary atomically — 715 lines,
   ownership, lock, staged atomic replacement and outcome states. Risk: item 5, isolated so a
   review can take it alone.
4. `c85580f` feat(tui): notify when a newer release is available — 446 lines, config keys and
   the one-shot TUI check with their tests.
5. `bb7ab9e` feat(upgrade): add the gojira upgrade command — 738 lines, the command, its tests
   and the README section it documents. Risk: item 5.

Deliberate deviation from the plan's numbering: the config/TUI slice landed before the command
slice, so the whole README travels with the command it documents instead of being split across
two commits by hunk.

Every slice is above the ~400-line guidance, as agreed when the split was approved: the unit is
one cohesive feature whose risky half (slices 3 and 5) is isolated, and splitting further would
have meant moving already-proven code between files for no review benefit. Nothing is pushed,
merged or released, and issue #41 remains open.

L7 (external review finding on PR #43, `cmd/tui.go`): configuration writes used an
unserialized read-modify-write — `Config.Save()` rewrites the whole file, so a concurrent
writer (for example `gojira config set` in another terminal) could be lost between the load
and the save. The reviewer's literal suggestion — reuse the config loaded at TUI startup
instead of reloading — was rejected: reusing a startup snapshot widens the loss window instead
of closing it, because every later write still races against the stale snapshot. Fix: one
serialized mutation entry point for all config writers, `config.Update(mutate)` in
`internal/config/write.go`. It creates the config directory, takes an exclusive
`config.yaml.lock` (`os.OpenFile` with `O_CREATE|O_EXCL|O_WRONLY` 0600, retrying every 10ms up
to `lockWaitTimeout` = 5s, then an error wrapping the exported `ErrWriteLocked`; a lock whose
mtime is older than `lockStaleAfter` = 30s is removed and the attempt retried, so a crashed
writer cannot block configuration forever; symlinks are inspected with `Lstat` and never
followed), then `Load()` → `mutate` → `Save()`, releasing the lock on every path — so a
concurrent caller's mutation is applied to the state the previous one wrote and no write is
lost. `cmd/config.go` now validates the key name first (an unknown key still fails without
touching disk with the same message, which lists `update_check`), keeps `token` on the
`config.SaveToken` path, and performs the mutation through `config.Update`. `cmd/tui.go`
replaces the inline recorder closure with the package-level `recordUpdateCheck(latest)` next to
`runTUI`, implemented as `config.Update` recording the timestamp and ignoring the returned
error exactly as the closure ignored failures.

RED/GREEN evidence (strict TDD, one RED per rule observed before its implementation; each run
under two seconds):
- Rule a, serialization: `go test -count=1 -run TestUpdateSerializesConcurrentWriters
  ./internal/config/` failed with `undefined: Update` (the unserialized/missing path); GREEN
  `ok ... 0.528s` after `write.go`, asserting B cannot complete while A holds the mutation and
  both mutations survive in the loaded config.
- Rule b, held lock: RED `undefined: lockWaitTimeout`, `undefined: ErrWriteLocked`; GREEN with
  `lockWaitTimeout` shrunk to 50ms and restored via `t.Cleanup` — `Update` returns an error
  wrapping `ErrWriteLocked` and the config file bytes are unchanged.
- Rule c, stale lock: RED `undefined: lockStaleAfter`; GREEN — a lock file aged past
  `lockStaleAfter` with `os.Chtimes` is reclaimed, `Update` succeeds and the lock is removed.
- Rule d, mutate error: RED `undefined: Update`; GREEN — the sentinel error propagates via
  `errors.Is` and the config file is not written.
- Rule e, TUI regression: `go test -count=1 -run TestRecordUpdateCheckPreservesUnrelatedValues
  ./cmd/` failed with `undefined: recordUpdateCheck`; GREEN `ok ... 0.343s` —
  `recordUpdateCheck("v9.9.9")` writes `update_last_check` (valid RFC3339) and
  `update_latest_version` while preserving the unrelated pre-existing `email`, `domain` and
  `board` values.

Suite and static checks: `go test -count=1 ./internal/config/ ./cmd/ ./internal/tui/` all ok;
`go test ./...` all packages ok; `go vet ./...` clean; `golangci-lint run` → 0 issues;
`gofmt -l internal/config cmd` empty. Differential against a binary built from HEAD (via
`git archive` into /tmp, nothing staged or committed): `config set` and `config get` for
`domain`, `email`, `board`, `project` and `people.<alias>`, plus an unknown key and an invalid
`update_check` value, produced byte-identical stdout, stderr, exit codes and `config.yaml` on
base and change, and left no `.lock` file behind. `gojira upgrade`, the public JSON contracts
and the TUI model are untouched.
