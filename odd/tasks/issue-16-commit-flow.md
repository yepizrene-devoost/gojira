# Issue #16 — Document the commit workflow

Status projection for the commit-flow documentation work unit.
Checkbox convention: `- [ ]` open, `- [x]` closed.

## Goal

Provide one canonical, scannable reference for splitting work into commits,
authoring commit messages, protecting branch boundaries, and relating commits
to review boundaries.

Branch: `feature/issue-16-commit-flow` (from `develop`, dflow, `--no-push`).

## Scope

Documentation only. No commit hook, CI check, generator, or production-code
change is included in this work unit. `.agents/workflows/commits.md` is the
repository reference; future automation may generate it from a configuration
source without changing the documented rules.

## Tasks

- [x] T1 — Write `.agents/workflows/commits.md` with the quick path, work-unit
      rules, conventional message format, branch-first guardrail, review-boundary
      guidance, and a pre-commit checklist.
- [x] T2 — Verify Markdown structure and repository status; record evidence.
- [x] T3 — Commit the documentation work unit and record its identity.

## Verification

- `git diff --check` passes.
- Markdown headings and tables are readable by inspection.
- No files outside the documentation and ODD evidence are changed.
- Runtime harness: N/A — documentation-only change has no runtime boundary.

## Evidence log

| Task | Commit | Verification |
|---|---|---|
| T1 | `5fbca26` | `.agents/workflows/commits.md` written; Markdown structure inspected |
| T2 | `5fbca26` | `git diff --check` passed; verification subagent found no readability issues |
| T3 | `5fbca26` | `docs(workflow): document the commit flow` created the work-unit commit; this follow-up records its identity |
