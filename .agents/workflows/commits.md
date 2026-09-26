---
name: commits
description: "Commit workflow: work-unit boundaries, message conventions, review boundaries, and delivery guardrails."
metadata:
  source: repository commit policy
---

# Commit Workflow

A commit is a reviewable work unit: one logical change with its tests and
user-facing documentation. Keep commits small enough to understand, verify,
and revert independently.

## Quick path

1. Start a work branch before committing when the current branch is `develop`
   or another protected/default branch.
2. Define one outcome for the work unit.
3. Keep implementation, tests, and directly related documentation together.
4. Verify the work unit, inspect its diff, and commit it with a Conventional
   Commit message.
5. Treat the resulting commit as the next review candidate and the previous
   reviewed boundary as its base.

## Work-unit rules

| Rule | Practice |
|---|---|
| One logical change | Do not mix unrelated fixes, cleanup, or formatting. |
| Complete the outcome | Include tests and docs required to understand or use the change. |
| Reviewable size | Split independent outcomes into separate commits; do not shrink a diff by removing useful coverage or documentation. |
| Independent rollback | A reviewer should be able to revert the commit without taking unrelated work with it. |
| Evidence with the unit | Record the focused verification and any runtime scenario before committing. |
| Honest history | Do not use fixup commits to hide incomplete work; correct the work unit before review. |

### Split by outcome, not by file type

Prefer commits that tell a complete story:

```text
feat(search): add server-side JQL filtering
fix(search): preserve quoted JQL terms

test(search): cover empty and quoted filters
```

Do not split the same behavior into separate “models”, “services”, and “tests”
commits when none of those commits works independently. Tests for a behavior
belong in the commit that introduces or changes that behavior.

## Commit messages

Use this format:

```text
 type(scope): concise description
```

The leading space above is illustrative only; the actual message starts at
`type`.

| Component | Rule |
|---|---|
| Type | One of `feat`, `fix`, `docs`, `style`, `refactor`, `test`, or `chore`. |
| Scope | Optional, short, and useful to identify the affected area. |
| Description | English, lowercase, imperative or outcome-oriented, and concise. |
| Punctuation | No final period. |
| References | Do not put ticket IDs or file paths in the subject. |

Examples:

```text
docs(workflow): document the commit flow
fix(jira): encode JQL query parameters
refactor(config): separate token storage from resolution
```

The body may explain motivation, constraints, verification, and follow-ups
when the subject is not enough. Keep the subject stable and scannable; put
implementation detail in the body or the diff.

## Branch boundary

Use `dflow` for branch lifecycle operations. Never create a work-unit commit
on `develop` or another protected/default branch when a feature or bugfix
branch is appropriate.

```bash
dflow start feature <name> --no-push
# implement and verify one work unit
git diff --check
git diff --stat
git commit -m "type(scope): concise description"
```

The branch workflow and merge targets remain defined by `.dflow.yaml` and
`.agents/workflows/dflow.md`. This document governs commit boundaries, not
branch naming or merge authorization.

## Review boundaries

Receipt-driven review operates on a commit or PR slice, not on an accumulated
working tree with unrelated later work.

| Situation | Boundary |
|---|---|
| New work unit | Review the new commit against the previous reviewed commit/tree. |
| Reviewed and approved commit | The approved commit becomes the next reviewed boundary. |
| Correction after review | The correction creates a new candidate; review the changed commit/tree again. |
| Stacked or chained work | Review each slice against its immediately preceding reviewed boundary. |
| Documentation or bookkeeping after approval | Keep it before the freeze when it belongs to the unit; otherwise it creates a new candidate. |

A review approval is not a commit, merge, push, or release authorization.
Those actions remain separate decisions under the repository workflow.

## Pre-commit checklist

- [ ] The commit has one clear outcome.
- [ ] Tests and directly related documentation are included.
- [ ] Unrelated changes are excluded.
- [ ] `git diff --check` passes.
- [ ] The focused verification command and result are known.
- [ ] Runtime verification is recorded, or `N/A` explains why no runtime boundary exists.
- [ ] The rollback boundary is clear.
- [ ] The message follows the format and allowed types above.
- [ ] The candidate boundary is clear for review.

## Related references

- `.agents/workflows/dflow.md` — branch types, merge targets, and finish rules.
- `AGENTS.md` — repository-wide agent instructions and commit conventions.
- `odd/tasks/` — work-unit task status and evidence logs.
