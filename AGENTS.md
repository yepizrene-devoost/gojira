# AGENTS.md

Repository instructions for coding agents working in this project.

## Session Start

- At the beginning of every session in this repository, before the first
  substantive reply, recover the working state:
  1. Read the recent memory context (`mem_context`, project `gojira`) and the
     latest session summary — that is where the previous session recorded
     decisions, discoveries and pending steps.
  2. List the open backlog: `gh issue list --state open`.
- Open the conversation with one line on where the last session left off and
  the current backlog, then ask what to pick up. Do not start work without
  that anchor.

## Language

- All documentation, code comments, commit messages and GitHub issues are
  written in **English**.

## Commit Messages

- Preferred format: `type(scope): description`
- Scope is optional when it does not add clarity: `type: description`
- Example: `feat(search): promote quick filter to server-side JQL`
- Use conventional commit types only: `feat`, `fix`, `docs`, `style`,
  `refactor`, `test`, `chore`.
- Keep the description concise and lowercase.
- Do not invent a ticket prefix when the branch name and recent repository
  history do not use one.
- Do not include full or relative file paths in commit messages.

## Branch Workflow

- Use `dflow` to manage branches, following `.dflow.yaml` and
  `.agents/workflows/dflow.md`.
- `feature/*` and `release/*` branch from `develop`; `hotfix/*` from `main`.
- `develop` merges directly (`auto`); `main` requires a PR (`manual`). Never
  suggest a PR toward an `auto` target.

## Security — Credentials

- `.env` holds real credentials locally and is gitignored. **Never** commit
  it, and never run `git add -A` without checking `git status` first.
- Never print API tokens in output, logs, issues or commit messages.
- Token storage/resolution rules live in `internal/config/config.go`:
  env vars > config file (OS keychain, YAML `chmod 600` fallback).

## Architecture Constraints

- **Single TUI model.** `internal/tui` is one Bubble Tea model with plain
  string-state switching — no sub-model message routing. Keep it that way.
- **Endpoints are centralized** in `internal/jira/paths.go`. Never hardcode a
  REST path anywhere else; that file is the single source of truth for the
  API surface (Platform v3 + Agile 1.0).
- **Agent JSON is a stable contract.** Curated output (`TicketJSON` and the
  `--json` commands) must stay valid JSON on stdout with diagnostics on
  stderr. Breaking field changes require a deliberate, documented decision.
- **Descriptions/mentions are ADF.** Atlassian returns Atlassian Document
  Format; flatten it in `internal/jira/types.go`, do not parse it in the TUI
  or CLI.

## Relevant Skills

When available, load these skills while working in this project:

| Skill | Applies to |
| --- | --- |
| `go-testing` | Go tests, `go test` coverage, Bubble Tea teatest |
| `work-unit-commits` | reviewable work-unit commits |
| `chained-pr` | stacked/chained PRs (slices under 400 lines) |
| `cognitive-doc-design` | docs with low cognitive load |
| `dflow` (workflow file) | branch lifecycle: start/finish/status |

## Source Of Truth

- `.agents/workflows/dflow.md` — branch types, merge rules, finish flow
  (generated from `.dflow.yaml`; regenerate with `dflow agent`).
- `internal/jira/paths.go` — every Jira REST/Agile endpoint.
- `README.md` — user-facing commands and keyboard shortcuts; update it in the
  same work unit that changes a command or key binding.
- Engram project memory (`gojira`) — roadmap phases (F1–F10) and decisions;
  the durable record of phase order and rationale.

## Closure and External-Action Confirmations

Closing an issue, merging, pushing, deleting a branch, or publishing a
release is always a standalone decision with its own explicit confirmation.

- Never infer closure from tentative phrasing ("creo que lo podemos cerrar",
  "¿qué opinas?", "no sé"): tentative means ask, plainly and separately.
- Never bundle a closure into the description of an option, a question, or
  another approval. Choosing "do X now" authorizes X only — not X plus close.
- The confirmation names the exact action and the state it changes ("cierro
  el issue #3 como completed"); execution waits for a plain, standalone yes.
