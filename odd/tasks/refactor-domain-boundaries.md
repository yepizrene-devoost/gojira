# Refactor — domain boundaries (SOLID audit follow-up)

Behavior-preserving refactor of the code added for #23, answering a SOLID audit. No public contract changes: CLI output, `--json` shapes, MCP tool list, schemas, descriptions and results stay byte-identical. Branch `feature/refactor-domain-boundaries`. Nothing is staged, committed, reviewed or merged without an explicit user instruction.

## Specs

- S1: Project-context logic leaves the CLI layer. A new package `internal/projectctx` owns the resolution policy (flag > `GOJIRA_PROJECT` > Git > user default), key validation, the `ResolvedProject` and `ProjectValidationError` types and the `ReadContext` use case (resolve plus authenticated identity). `internal/config` keeps only persisted configuration and tokens and no longer imports `os/exec`. The Git reader is an infrastructure adapter inside `projectctx`, injected behind a small interface; `cmd/context.go`, `cmd/init.go` and the MCP tool call `projectctx`. The existing behavior and tests (precedence, real Git scopes, worktree, submodule, outside a repository) move with the code and keep passing.
- S2: The MCP adapter stops being a 14-case `switch` inside one closure. Each tool is a small value with its own name, description, input schema, validation and handler, registered in a table, so adding a tool touches one place. Tool names, order, descriptions, input schemas, annotations, the guide text, results and error envelopes stay identical. The adapter lives in `internal/mcpserver` and receives its Jira access and its error mapper by injection; `cmd/mcp.go` only wires it (the `newMCPServer(client, domain)` constructor used by the tests is kept).
- S3: `internal/jira/client.go` (916 lines, 44 methods) is split by domain inside the same package: transport (`Client`, requests, retries, errors), issues, boards and sprints, search, projects, users and identity, worklogs and comments. Pure moves: no exported name, signature or behavior changes, and the exported API printed by `go doc -all ./internal/jira` stays identical.
- S4: Consumers depend on small interfaces defined where they are used (for example identity and project reads for `projectctx` and `init`), not on the concrete `*jira.Client`, wherever S1 and S2 touch them. No interface is created for its own sake.
- S5: Verification is differential: the exported API of `internal/jira` and `internal/config` (before/after `go doc`), the MCP `tools/list` and responses over stdio (hashes) and the CLI output of `projects`, `projects --json` and `context --json` against a build of `develop` (commit 2e11aea) on the real Jira, read-only. `go vet`, `go test ./... -count=1`, `golangci-lint run ./...` and `-race` on the touched packages pass; existing tests are only moved or adapted to the new package boundaries, never weakened.
- S6: Non-goals: no new feature, no endpoint change, no change to `internal/tui` (single TUI model constraint), no change to JSON schemas or error codes, no removal of the test seams `buildClient` and `isTerminal` unless S4 needs it.

## Tasks

- [ ] R1 (S1, S4): `internal/projectctx` and call-site migration (second of three commits).
- [x] R3 (S3): `internal/jira/client.go` 916 -> 228 lines plus six domain files. Pure move: the 120 top-level declarations are identical (exact diff, verbatim text) and `go doc -all ./internal/jira` is identical.
- [ ] R2 (S2, S4): `internal/mcpserver` tool table, with the characterization tests first (third of three commits).
- [ ] D (S5): differential and full verification, independent check, README structure and the user's review (closed in the third commit).

Risk: item 3 (the MCP tool surface and the exported client API are contracts others consume; a regression would be silent) and item 6 is mitigated by the differential checks. No data is rewritten.

## Log

- L1: "si haz el refactor", answering the question "¿hago esa refactorización como una unidad aparte antes de seguir con reportes, o prefieres seguir con funcionalidad y dejarla anotada como deuda?" The audit behind it was the user's question: "estas siguiendo solid de forma correcta para este project? dividiendo en dominios cada cosa que vas creando?"
- L2: Audit evidence (develop 2e11aea): `internal/jira/client.go` 916 lines and 44 methods; `cmd/mcp.go` `newMCPServer` 102 lines with a 14-case switch (adding a tool touches four places); `readContext` lives in `cmd/context.go` and the MCP adapter calls it; `internal/config/project.go` imports `os/exec`; consumers depend on the concrete `*jira.Client`. `internal/tui/tui.go` (2,951 lines) is a pre-existing AGENTS.md constraint and is out of scope.
- L3: Baseline captured before any change, from a temporary Git repository with `gojira.project = ARA` (not the user's repositories): a build of develop at `/tmp/gojira-base2`, `go doc -all` of `internal/jira` and `internal/config`, an MCP stdio smoke (initialize instructions, `tools/list`, guide resource and eight tool calls, as hashes) that is deterministic across two runs, and the CLI output of `projects`, `projects --json` and `context --json`.
- L4: Both first-round workers stopped asking for write access outside their allowed edit surfaces (scratch files and a verification directory). Decision: workers write only inside their surfaces; the parent prepares the baselines beforehand and runs every differential itself, and workers compare read-only (stdout diffs).
- L5: The parent's first exact declaration diff failed because the extractor keyed receivers with file positions (a defect of the parent's tool, not of the move). Fixed (keys by receiver name) and the baseline was retaken from `git archive HEAD`, not from the live tree.
- L6: First of three commits (the user chose three stage, freeze and commit cycles): the Jira client split by domain (R3), a pure move.
