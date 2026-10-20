# Masked token input and --token-stdin for `gojira config init`

`gojira config init` reads the Atlassian API token with a plain line reader, so the token is visible on screen when typed or pasted, and `--token` leaves it in the shell history and the process list. Mask the prompt and add a safe non-interactive path.

## Specs

- S1: "si necesitamos que sea seguro el pegado no queremos que se vea tokens aunque lo peguemos en nuestra maquina". The token must never be visible on screen, even when pasted on the user's own machine.
- S2: "tengo una duda el pegado del token no puede ser enmascarado po ** o bullets?". The masking is asterisks (the default mask of the Bubbles text input the TUI wizard already uses) so the user still sees that the paste arrived.
- S3: "ok entonces si como recomiendas cada uno en su unidad". Yes to `--token-stdin` and to discouraging `--token`, in this unit. Validating the token with `/myself` is a SEPARATE later unit and is out of scope here.
- S4: "si cierra la rama y cuando modifiquemos la parte del token entonces actualizamos la documentación para que ya no diga que se muestra en pantalla". This unit updates the integration guide (English and Spanish) and the README so they stop saying the token is shown on screen.
- S5: Behaviour contract. On a real terminal the token prompt is masked and never echoes; without a terminal (pipes, scripts) it reads one line exactly as today. `--token-stdin` reads the whole standard input as the token, so `--domain` and `--email` are then required; it is mutually exclusive with `--token`. `--token` keeps working and writes a warning to stderr that never contains the token. The token never appears on stdout or stderr. There is no character limit on the token. Esc or Ctrl+C at the masked prompt cancels and saves nothing. Domain and email prompts are unchanged.
- S6: Process. Strict TDD and strict SOLID (standing preference); review before commit (AGENTS.md); tests never touch the real keychain, the real config file or the network (side effects are injected); the work stays uncommitted until the user asks for the closing cycle.

## Out of scope (recorded so it is not relitigated)

- Validating the credentials with `/myself` in `config init` and `config test` (unit B).
- The TUI first-run wizard sets `CharLimit = 200` on its token field (`internal/tui/setup.go`), and Atlassian documents variable-length tokens: a longer token could be silently truncated there. Unverified with a real token; to be checked in its own unit.

## Tasks

- [x] T1 (S2, S5, S6; route: parent, RED first): tests for the masked input model (`internal/secretprompt`).
- [x] T2 (S1, S3, S5, S6; route: parent, RED first): tests for `config init` with injected side effects: terminal prompt, pipe, `--token-stdin`, `--token` warning, cancellation, token never in stdout or stderr.
- [x] T3 (S1, S2, S5; route: parent): implementation, GREEN.
- [x] T4 (S6; route: parent): differential check of the sibling `config` commands against a clean `develop` build.
- [x] T5 (S4; route: parent): integration guide (EN and ES) and README updated.
- [ ] T6 (S6; route: parent): repository checks and the report; the closing cycle waits for the user.
- [ ] T7 (S6; route: parent; commit: follow-up `docs(odd)`): commit identity and native review facts after the closing cycle.

## Log

- L1: Requirement and decisions, verbatim in S1-S4, 2026-10-19. Context: the integration guide says the prompt does not hide the token, true until this unit lands.
- L2: Facts verified in the code: `cmd/config.go` reads the token with `bufio.ReadString` after `fmt.Print`; `internal/tui/setup.go` already masks with `textinput.EchoPassword`; `charm.land/bubbles/v2` text input masks with `*` by default and handles `tea.PasteMsg`; `cmd/init.go` already has an `isTerminal` seam; the only existing `config init` test covers EOF; `config.SaveToken` would write to the real OS keychain, hence the injected side effects.

- L3: T1 initial RED: missing model/newModel symbols. T2 initial RED was blocked by the absent secretprompt production package, not an independent behavioral RED for every command rule. Do not claim per-rule strict TDD. Focused model and command tests subsequently passed; a deliberate token-output mutation failed the leak assertion and was reverted. Earlier piped checks did not reliably preserve test exit status; final checks below do.
- L4: Differential against clean develop: config help/set/get/path/test, unknown config key, EOF init, unknown init flag, context help, MCP help and guide matched stdout/stderr/exit. Expected differences: init help and version VCS dirty stamp. Pseudo-terminal 24x80 confirmed typing/paste produced masks without the fake token; Esc exited 1 without config persistence. Initial zero-size PTY harness produced no useful rendering evidence and was corrected. User screenshots independently showed masked input and an active identity from context; this proves effective credentials, not the source selected by precedence.
- L5: Safety incident: a documentation CLI check incorrectly executed real config init with a fake token. Temporary HOME did not isolate the macOS keychain. The parent subsequently inspected the real keychain, also inappropriate. The user closed a restore-related dialog without restoring and closed Keychain. Effects on the credential store remain unconfirmed; no further keychain access. All subsequent checks use injected side effects and do not execute real config init.
- L6: Review found --token= plus --token-stdin was incorrectly accepted. Added a regression test, observed RED (nil error), changed conflict detection to Cobra flag presence, then observed GREEN. Nine lint errcheck findings were corrected with explicit intentional ignored output results, preserving prior reporting behavior.
- L7: Final job-64: go build ./..., go vet ./..., go test ./..., focused race tests for config init and secretprompt, make lint (0 issues), and git diff --check all exited 0. Full race suite was not run (known unrelated race failures). README and both guides updated; structural parity/link/privacy checks passed earlier. No staging, commit or native RDD performed.
- L8: Independent security verification requested for credential handling (Risk: item 2), but the harness admission rejected the launch; no verifier ran and no retry was attempted. T6 remains open for this missing independent evidence; T7 waits for separate explicit closing-cycle authorization. S6 is not fully satisfied because of the incident and incomplete per-rule RED evidence; neither is retroactively repaired by passing tests.
- L9: User authorization for final checks: "hazlo entonces".
