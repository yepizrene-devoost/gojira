# GoJira integration guide

**English** · [Español](integration-guide.es.md)

GoJira talks to Jira Cloud with **your own Atlassian API token**, keeps that token on your machine, and works from a terminal, from an AI agent that can run commands, or from any AI client that supports MCP. This guide takes you from nothing to a working setup. You do not need to be a programmer; you only need to be able to open a terminal.

## Quick path

| # | You do | You should see |
|---|---|---|
| 1 | [Install GoJira](../README.md#installation) | `gojira version` prints a version |
| 2 | [Create an Atlassian API token](#2-create-an-atlassian-api-token) | a long token, shown **once** |
| 3 | `gojira config init` | the prompts for domain, email and token |
| 4 | `gojira context` | a line `Identity: <your display name> (active: true)` |
| 5 | `gojira projects` | the projects you can see |

> **Why step 4 and not `gojira config test`?** `config test`, and the "connected" line printed by `config init`, call Jira's server-info endpoint, which answers without credentials. They confirm that the **site address** is right, not that the **token** is valid. `gojira context` asks Jira who you are, so it fails when the email or token is wrong.

## 1. Install

Follow [Installation](../README.md#installation) in the README: one command for Linux and macOS, one for Windows PowerShell, and no Go toolchain needed. Read [Installer trust and integrity](../README.md#installer-trust-and-integrity) if you prefer to inspect the script first.

## 2. Create an Atlassian API token

1. Sign in with the Atlassian account you use for Jira and open <https://id.atlassian.com/manage-profile/security/api-tokens>.
2. If you sign in with a password or a third-party login, Atlassian may email you a one-time code to verify your identity first.
3. Choose **Create API token**. Choose the plain one, **not** the "with scopes" option: Atlassian documents a different base address for scoped tokens, and GoJira calls your site address (`https://<your-site>.atlassian.net`).
4. Name it for what it does (for example `GoJira laptop`) and pick an **expiration date**. Atlassian allows 1 to 365 days and defaults to one year.
5. Choose **Copy to clipboard** and store the token in a password manager. **Atlassian does not show it again.**

| Good to know | Detail |
|---|---|
| Treat it like a password | Anyone holding the token can call Jira as you. Never put it in a commit, an issue, a chat or a screenshot. |
| It expires | When it does, requests are rejected. Create a new token and run `gojira config init` again. A calendar reminder helps. |
| You can revoke it | On the same page, choose **Revoke** next to the token whenever you stop using it. |

## 3. Configure GoJira

Run the setup once:

```bash
gojira config init
```

It asks for three things:

| Prompt | What to enter |
|---|---|
| `Jira domain` | Your site domain **without** `https://`, for example `mycompany.atlassian.net` |
| `Jira email` | The email of the Atlassian account that created the token |
| `Jira API token` | The token you copied |

> The token prompt does **not** hide what you type or paste. Do it where nobody can see your screen, and clear the terminal afterwards.

Then verify it with `gojira context`. Without a selected project it prints `Project: not configured; ask the human for a project.` on the first line; that is normal. What matters is the second line, `Identity: <your display name> (active: true)`: it proves the email and token are valid.

**Where things are stored**

| Item | Where |
|---|---|
| Domain and email | `~/.config/gojira/config.yaml` (run `gojira config path` to see the exact location) |
| API token | Your operating system's keychain. If no keychain is available it falls back to the same YAML file, requesting mode `0600` when the file is created; **that file is not encrypted**. |

**Environment variables instead of the wizard** (useful for scripts and CI) take precedence over the stored configuration:

```bash
export JIRA_DOMAIN="mycompany.atlassian.net"
export JIRA_EMAIL="you@example.com"
export JIRA_API_TOKEN="..."
```

Avoid `gojira config init --token <TOKEN>` on shared machines: shell history keeps the whole command line. GoJira also reads a `.env` file in the folder where you run it.

## 4. Use it

| I want to… | Run | More |
|---|---|---|
| Work in a terminal | `gojira tui` for the board, or commands such as `gojira search "assignee = currentUser()"` | [CLI commands](../README.md#cli-commands), [TUI shortcuts](../README.md#tui-keyboard-shortcuts) |
| Let an AI agent that can run commands use it | Add `--json` to a command: the agent gets stable JSON on stdout and diagnostics on stderr | [Agent usage examples](../README.md#agent-usage-examples), [TicketJSON v1](../README.md#ticketjson-v1-contract) |
| Connect an AI client through MCP | `gojira mcp config`, then paste the printed entry into the client | [Read-only MCP server](../README.md#read-only-mcp-server) |
| Give an agent usage guidance without files | `gojira guide` prints it; the MCP resource `gojira://guide` serves it | [Optional agent guide](../README.md#optional-agent-guide) |

### Connect an AI client (MCP)

Any MCP client that can launch a local command can use GoJira. Let GoJira print the entry for you, already pointing at your copy of the program:

```bash
gojira mcp config                  # the common mcpServers JSON (default)
gojira mcp config --client codex   # the Codex config.toml entry
```

- The output contains **no credentials**. The server reads them from your `gojira config init` setup, so the snippet is safe to paste or share.
- By default the agent can **only read**. Add `--allow-writes` to the command **only if** you want the agent to create tickets, comment, log work or move issues with **your** account: see [Opt-in guarded writes](../README.md#opt-in-guarded-writes).
- Ticket text is untrusted data. Keep write access for agents and clients you trust.
- Where to paste the entry depends on the client. The README lists the Codex locations; for other clients, check their documentation for the MCP configuration file.

## 5. Safety checklist

- [ ] The token lives in a password manager, not in a note, a commit or a chat.
- [ ] I picked an expiry and know when to renew it.
- [ ] I revoke tokens I no longer use.
- [ ] I use the prompt or environment variables instead of `--token`.
- [ ] My MCP entry is read-only unless I need writes.

## Troubleshooting

| What you see | Likely cause | What to do |
|---|---|---|
| `no credentials found (set env vars or run: gojira config init)` | Nothing is configured, or only some environment variables are set | Run `gojira config init`, or set all three variables |
| `incomplete credentials (run: gojira config init)` | The domain, email or token is empty | Run `gojira config init` again |
| `connection failed: … no such host` | Wrong domain or no network | Use `mycompany.atlassian.net` without `https://` and check your connection |
| `config test` says connected but other commands fail with an authorization error | Wrong or expired token, or the email does not belong to the token | Run `gojira context`; create a new token and run `gojira config init` again. See [Jira access errors for agents](../README.md#jira-access-errors-for-agents) |
| `MCP credentials unavailable; configure credentials with gojira config init` | The MCP server cannot find credentials | Run `gojira config init` as the same operating-system user that launches the client |
| Results come from the wrong account or site | `JIRA_*` variables or a `.env` file in the current folder override the stored setup | Check your environment and the folder where you run GoJira |

## Next steps

- [CLI commands](../README.md#cli-commands) and [Search completeness](../README.md#search-completeness)
- [Project context](../README.md#project-context): remember a Jira project per repository clone
- [User candidates and time reports](../README.md#user-candidates-and-time-reports)
