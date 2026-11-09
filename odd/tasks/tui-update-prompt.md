# Interactive update prompt in the TUI

Issue: https://github.com/yepizrene-devoost/gojira/issues/47 — branch `feature/issue-47-update-prompt`.
Scope: check every start with a conditional request, and replace the passive notice with a centred modal that can install.

## Specs

**S1 — Check on every start.** The TUI checks the latest release on each start unless `update_check` is `false`. No time gate by default: `update_check_interval` accepts a duration and `0` means every start.

**S2 — Conditional request.** The check stores the response `ETag` in `update_etag` and sends it as `If-None-Match`. A `304` means "no new release": the stored `update_latest_version` is reused, no comparison artefacts change, and the call does not consume GitHub's rate limit. A `200` refreshes both `update_etag` and `update_latest_version`.

**S3 — Provenance.** A check resolves to one of three states: `Completed` (it ran and produced a version), `Skipped` (`update_check` disabled, `dev` build, or inside a configured interval) and `Unsuccessful` (network or API failure). Only `Completed` may assert freshness; the others never claim "up to date" and never show the modal.

**S4 — Dev builds skip.** A binary whose version is `dev` never checks: comparing `dev` with a tag is meaningless and the installer refuses those binaries.

**S5 — Never re-notify the same version.** When `update_latest_version` already equals the newer version the user dismissed or viewed, the modal does not reopen on the next start.

**S6 — Centred modal.** The prompt is a modal built with the existing helpers (`modalStyle`, `composeOverlay`, `modalWidth`/`modalHeight`) so it inherits the current frame and centring. It stays inside the terminal at narrow and wide widths, wraps its lines, and never renders outside bounds.

**S7 — Three options.** The modal lists `gojira <installed> → <latest>`, the release URL, and the options **Update now**, **View changes**, **Keep current version**. The cursor starts on `Keep current version`. Keys: `↑`/`↓` move, `enter` confirms, `esc` keeps, and the shortcuts `u` (update now), `v` (view changes), `c` (keep) act directly.

**S8 — Update now installs.** `u` or confirming `Update now` runs the existing upgrade flow in place, showing progress, then reports the outcome. It must surface every refusal the installer already produces — development build, package-manager path, `go install` location, symlinked executable or directory — inside the modal instead of failing silently.

**S9 — View changes.** `v` opens the release URL in the browser through the existing helper, and leaves the modal open.

**S10 — Status line.** When a newer version exists, the boards header shows it so it stays visible after the modal closes; the line disappears once the user updates or the running version is current.

**S11 — Docs.** CHANGELOG and README stop claiming that the TUI never installs anything and describe the new prompt and configuration keys in the same work unit.

## Tasks

- T1 — Config and check layer (S1–S5): `internal/config/config.go`, `internal/upgrade/check.go` + tests. **done** — RED per rule (missing symbols), GREEN 15 tests; ETag/304/200, interval fail-safe, dev zero requests via counting RoundTripper. commits: f275db8, 8cf05cf.
- T2 — TUI modal (S6, S7, S9, S10): `internal/tui/tui.go`, `internal/tui/update_prompt_test.go`. **done** — RED per rule (no modal, no cursor on Keep, keys dead, no URL opened), GREEN; centred with shared modal helpers, safe default, no re-notify, status line. commit: 888b347.
- T3 — Install path (S8): `internal/tui/tui.go`, `internal/tui/update_install_test.go`. **done** — RED per rule (placeholder toast, missing refusal texts, no busy guard), GREEN 9 tests; real pipeline stages, refusals in-modal, MutationUnknown honesty, restart note. commit: 7a8577d.
- T4 — Docs (S11): `README.md`, `CHANGELOG.md`. **done** — structural verification only; both stop claiming the TUI never installs and document the new prompt, keys and interval. commit: cf8a6c8.
- T5 — Verification: **done** — `go build ./...` OK, `go vet ./...` OK, `golangci-lint run ./...` 0 issues, `go test ./... -count=1` 14 packages ok and zero failures, `tests/release-contract.sh` passes, gofmt clean outside the pre-existing `internal/tui` baseline. No separate commit: verification accompanies each slice.

## Log

- L1 (user, verbatim): "y con respecto a lo del aviso de actualización en el tui porque no aparece?"
- L2 (user, verbatim): "osea que el aviso en la tui unicamente es aviso no da la opción de actualizar desde la tui???? y no deberia ser cada 24 horas si no cada vez que se abre la tui, o no se si exista una forma \"barata\" de verificar si hay una actualización, porque si no cada que inicie y por ejemplo si lance varios release en un dia no me actualizara"
- L3 (user, verbatim): "me parece la opción mas correcta la 3" — choose the conditional-request option: check on every start with `ETag`/`304`.
- L4 (user, verbatim): "sin embargo me gustaria replicar el comportamiento de actualización por tui de gentle-ai ... es una tui en go también entonces pienso que nos puede servir de inspiración analiza el codigo de la forma en la que actualizan en esa herramienta para replicar talvez el comportamiento pero obviamente con nuestra interfaz".
- L5 (user, verbatim): "me parece apropiada tu propuesta solo algo que aclarar quiero el modal centrado y ajustado a nuestra estetica de la tui, no quiero que aparezcan los modals desalineados y asi".
- L6 (user, verbatim, on reversing the v0.6.0 guarantee): "Si instalaremos desde TUI, como lo hace Gentle ai y tendremos que cambiar la documentación para reflejar esta decisión, no veo cual es el problema en que contradiga, ahora estamos cambiando de decisión y no pasa nada, lo hacemos en pro de la facilidad y no estamos haciendo nada malicioso asi que no tenemos problema en hacerlo asi y tomar esta nueva decisión".
- L7 (evidence, from gentle-ai `main`): its cooldown is `UpdateCheckTTL = 6 * time.Hour`, the check runs asynchronously after the interface opens with a spinner, results are not cached, only a successful check persists the timestamp, and the prompt offers Update now / View changes / Keep with the cursor on Keep. Its GitHub client sends `Accept: application/vnd.github+json` and `User-Agent`, optional bearer token, and no conditional request — so S2 is cheaper than the reference implementation.
- L8 (decision): S8 deliberately reverses the v0.6.0 statement that the TUI never installs. The installer path and its guards are unchanged; only the entry point is new.
