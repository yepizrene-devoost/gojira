# Issue #20 — remaining ticket CRUD and TUI parity

## Origin and boundary

This is the unimplemented remainder of [issue #9](https://github.com/yepizrene-devoost/gojira/issues/9), transferred at the user's request because #9 became too broad. The GitHub source of truth is [issue #20](https://github.com/yepizrene-devoost/gojira/issues/20). #9's approved work units remain historical evidence, not work to repeat. Issue #19 remains design-only for TUI detail UX; comments implementation belongs here. The handoff does not close #9, authorize a PR or release, or permit any real Jira deletion.

## Constraints

- Keep the single Bubble Tea model, centralized endpoints in `internal/jira/paths.go`, ADF conversion in `internal/jira/types.go`, and stable TicketJSON v1 stdout with diagnostics on stderr.
- Treat `applied`, `not_applied`, and `unknown` as distinct mutation states. Never retry an ambiguous write automatically. Only use local HTTP simulations for mutation acceptance tests.
- Split implementation into independently functional work units with tests, documentation, ODD evidence, local commit authorization, and native review when enabled. Do not treat a scope transfer as completed behavior.
- Preserve the selected product contracts from #9: Done-category children/total progress; backlog removal of future/active sprint assignments without erasing closed history; per-issue available transitions for close/reopen; permanent deletion only after explicit confirmation. Neither the decision to implement deletion nor this issue authorizes deleting a real ticket.

## Open work

- [ ] I20-1: Complete the all-command JSON/error audit. Check remaining commands including `version`, write-result readback and stable stderr/exit handling; document intentional non-JSON `config`/`tui` behavior. Preserve the existing approved TicketJSON v1 and raw Board/Project array shapes. Test failures with local HTTP and failing writers.
- [ ] I20-2: Show verified Done-category child progress on board cards and available JSON, plus sprint name/state on `get` and cards. Distinguish unavailable fields from genuine zero; avoid N+1 reads and do not infer undocumented Jira field shapes as fact.
- [ ] I20-3: Support remaining CLI/TUI sprint selection and movement, including TUI backlog movement beyond add-to-active. Preserve request scoping, partial/unknown write states, and safe refresh behavior; closed-sprint history must not be rewritten by a synthetic multi-write loop.
- [ ] I20-4: Complete CLI/TUI lifecycle parity: close/reopen through transitions actually available on the selected issue and permanent delete with a clear, separate confirmation. Test only with local HTTP; no real-ticket deletion without a new explicit action authorization.
- [ ] I20-5: Parse logged time/worklogs into TicketJSON, aggregate time on `get`, and complete TUI detail with readable dates, worklog/time summary, existing comments, and comment entry. Preserve ADF and detail navigation. Coordinate interaction design with #19 without treating that design issue as implementation authority.
- [ ] I20-6: Reconcile README, focused/full Go tests, local HTTP/TUI regression evidence, and the remaining CLI/TUI parity matrix before requesting delivery. Publishing, issue closure, tagging and release remain separate user decisions.

## Handoff evidence

Issue #20 was created and read back as OPEN. Issue #9 was annotated with the scope handoff and remains OPEN; issue #19's design-only references now point comment implementation here and it remains OPEN. The last approved #9 work-unit boundary was `891b896c109d0c5956522b90b40daab7624b3eed` after I9-2e (`review-edd8a4d836ca311b`, approved and acknowledged). This document only transfers future scope; it makes no claim that I20-1 through I20-6 have been implemented or reviewed.
