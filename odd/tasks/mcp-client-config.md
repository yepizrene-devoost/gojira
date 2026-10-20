# MCP client configuration for non-programmer users

Print a ready-to-paste MCP server entry for any AI client, so a non-programmer can use gojira without a repository or hand-edited configuration. Distribution track of issue 23; provider-neutral.

## Specs

- S1: "no se si mi pm use cloude desktop o no ni que use pero quiero que pueda usarse desde clientes de IA aunque no sean programadores". The solution must not depend on a specific client.
- S2: "y tampoco quiero atarlo a un solo proveedor gojira debe poderse usar casi de forma universal en medida de lo posible". Layered design: the core knows no vendor; client support is a thin, optional layer. No feature exists only in a client-specific shortcut. A new client costs one renderer entry.
- S3: "no igual implementemoslo y yo lo veo con el directamente en una llamada cuando le diga como usarlo". Implement now; real-client verification (including Windows and the desktop apps) happens in a call with the PM, so it is NOT claimed here.
- S4: `gojira mcp config [--client codex|json] [--allow-writes]` prints one entry pointing at the absolute path of the running binary. No credential is ever printed: `gojira mcp` reads them from `gojira config init`. The default is read-only; `--allow-writes` only adds the existing flag.
- S5: "no, asi esta bien yo pense que no era posible la escritura con los agentes, eso lo dejaste en la documentación?" then "si". Writes stay opt-in; the README must say that writing is possible and point to "Opt-in guarded writes".
- S6: "porque para el también quiero agregar la capa de oauth para que no tenga que crear api keys complicadas". OAuth is NOT part of this unit: it touches credentials and the API base (`api.atlassian.com` with a cloud id), so it stays with issue 4 pending a design decision (who registers the Atlassian app, how its secrets are handled).
- S7: "si sigue el ciclo de cierre". Native review, commit and merge into `develop` through dflow, each following the repository rules in AGENTS.md.

## Tasks

- [x] T1 (S2, S4; route: parent, strict TDD; commit: this unit): eight tests in `cmd/mcp_config_test.go` written first; RED observed as a build failure (`executablePath` and `newMCPConfigCommand` undefined); one test then needed `SilenceUsage`/`SilenceErrors` to mirror `rootCmd`. GREEN: `cmd/mcp_config.go` with a renderer map (`codex` TOML, `json` mcpServers), seam `executablePath`, escaping verified for Windows paths, error lists the supported clients, stdout stays empty on error.
- [x] T2 (S2; route: parent; commit: this unit): differential against a clean `develop` build over `mcp bogus`, `guide`, `context --help`, `get --help`, `version`, `mcp --help`: identical except `mcp --help` (lists the new subcommand, expected) and `version` (`-dirty` VCS stamp, expected).
- [x] T3 (S3, S5; route: parent; commit: this unit): README section "Read-only MCP server" documents `gojira mcp config`, where Codex reads its entry (official OpenAI docs, verified) and states that other clients are NOT verified; one sentence on `--allow-writes` linking "Opt-in guarded writes".
- [x] T4 (S7; route: parent; commit: this unit): `go build ./...`, `go vet ./...`, `go test ./...`, `golangci-lint run ./...` (0 issues) and `git diff --check` pass on the final tree. `gofmt -l` still lists `internal/tui/tui.go` and `internal/tui/tui_test.go`, pre-existing and untouched by this unit.
- [x] T5 (S7; route: parent; commit: follow-up `docs(odd)`): code commit `adeab05` (`feat(mcp): print ready-to-paste client configuration`), whose `HEAD^{tree}` equals the frozen candidate tree `04e14cc79c829766d1ab19842989751396c3d5e5` (index checked with `git write-tree` before committing, no unstaged diff). Native review `review-e2ed3e6e178a28e6` (medium, lenses risk + reliability, 5 paths, 289 changed lines): both lenses admitted zero findings, approved, acknowledged (authority burned). The merge into `develop` is a separate step that waits for the user's exact confirmation (L5).

## Log

- L1: User request, verbatim in S1-S3. The parent first researched how Codex, the ChatGPT desktop app, Cursor, Gemini CLI and Windsurf configure local MCP servers; only the OpenAI page was checked against official documentation, and a reported Windows bug (`openai/codex#38162`, tools discovered but not exposed in the ChatGPT desktop app) is unresolved and unverified here.
- L2: Finding that shaped the design: the MCP server reads credentials from the keychain, so the printed entry needs no secret. This removes the encrypted-credential injection risk noted for `.mcpb` bundles; a `.mcpb` shortcut for Claude Desktop remains a possible later thin layer.
- L3: A first draft of the README mentioned writes only inside a code comment; the user's question exposed it (S5) and the paragraph was added.
- L4: Pre-freeze declaration, 2026-10-19 (the freeze act and the declared expectation, never the verdict): the user confirmed the unit with "si sigue el ciclo de cierre" (S7). The five paths are staged explicitly, the new files included. The review is an ordinary workspace candidate with lenses `risk` (a command that emits configuration other tools will execute, and can add the write-enabling flag) and `reliability` (output formats other clients parse). Nothing about the review outcome is written in the tree. Publication and the merge are confirmed with the user separately, as AGENTS.md requires.
- L5: Delivery boundary, per AGENTS.md: publishing the branch, merging into `develop` and deleting the branch are standalone decisions. The user ordered "si sigue el ciclo de cierre" after a question that named review, commit and merge into `develop`; the push that `dflow finish` performs, and any branch deletion, are confirmed separately before running it. OAuth (S6) stays with issue 4.
