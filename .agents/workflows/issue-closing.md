# Issue closure

Closing an issue is a delivery act with its own checklist. It is never a side
effect of a branch operation: `dflow` finishes branches and merges them into the
configured targets, but it does not close issues, so nothing in the branch
workflow removes a stale `status:` label or writes a closing comment.

Use this checklist for every issue, in this order, after a human confirms that
exact closure.

## Preconditions

- Every tracked item of the issue is complete: implementation, merge into the
  configured target, and any tagging, publication or verification the issue
  names.
- The closure confirmation is explicit, standalone, and names this exact issue.
  Tentative phrasing is not confirmation, and a closure is never bundled with
  another approval.

## Steps

1. **Finish state.** Confirm no tracked delivery work remains. Check the merge
   commit and the target branch (`git log --oneline -3`, `git status`), and any
   release step the issue tracks.
2. **Labels.** Remove every `status:` label that names a state that has ended,
   keeping content and type labels:

   ```
   gh issue edit <n> --remove-label "status:approved"
   ```

   Repeat per ended label. There is no `status:completed` label in this
   repository, so nothing replaces it.
3. **Closing comment.** Record the delivery commit or merge commit, what landed
   beyond the original body when the scope grew, the checks that ran, and every
   open risk that remains. A comment is the durable record; do not rewrite the
   issue body to look complete.
4. **Close.** `gh issue close <n> --reason completed`.
5. **Verify and quote.** Run `gh issue view <n> --json state,stateReason,labels`
   and paste the result into the closure report. A closure report without that
   output is incomplete: the state, the reason and the label set are the three
   facts being asserted.

## Closing keywords

`Closes`, `Fixes` and `Resolves` can close an issue from a merge without any of
the steps above, silently skipping the label and comment steps. Prefer
`References #<n>` while work remains, and reserve a closing keyword for a case
where the merge itself fully satisfies the scope and a separate explicit
authorization approves closing that exact issue upon merge.

## Failure mode this prevents

An issue closed with `gh issue close` alone keeps `status:approved`, a label that
means "approved for implementation, PRs can now be opened". A closed issue that
still invites implementation is worse than an open one: the backlog view lies to
whoever reads it next.
