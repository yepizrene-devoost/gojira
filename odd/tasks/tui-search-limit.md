# TUI search limit parity
Issue #33 T8; branch `feature/mcp-parent-creation`. Preserve existing search default and semantics.
Work remains unstaged and uncommitted for maintainer review.

## Specs

- S1: "hay que implementar todo entonces"
- S2: "todo debe ser tdd"
- S3: "era crear todos los commits en una rama y al final hacer dflow finish --delete cada commit era una unidad de la misma rama"

## Tasks

- [x] T1 (S1-S3; route: worker; commit: c5b7d76): Add user-selectable search limit and CLI-equivalent all-results cap to TUI with strict TDD, preserving default behavior and truthful completeness.
- [x] T2 (S1-S3; route: parent; commit: c5b7d76): Verify and present unit without staging or committing.

## Log

- L1: "hay que implementar todo entonces"
- L2: "todo debe ser tdd"
- L3: "era crear todos los commits en una rama y al final hacer dflow finish --delete cada commit era una unidad de la misma rama"
- L4: Worker reported RED/GREEN for discoverability, limit selection, invalid values, mode routing, all input acceptance, estimate wording, paste, and guidance. The all RED stopped at rejecting the input and did not reach the cap assertion; no separate prior RED was observed for boundary or cancellation rules. Independent verifier found no blocking functional findings; focused and full suites, vet, and diff-check passed. Strict TDD evidence gaps remain open.
- L5: After being informed of the open TDD evidence gaps, user instructed: "haz commit de los cambios". This authorizes this commit only; T1 and T2 remain open and no RDD, merge or issue closure is implied.
- L6: User answered "si dale" to three questions: commit the ledger reconciliation; push the branch and open a PR to develop that references #33 without a closing keyword; and whether to accept the disclosed TDD caveats or redo them. Recorded as acceptance of the caveats listed in this ledger for this unit only. Strict TDD stays mandatory for future work. The caveats are the missing prior RED for the all-mode 10,000 cap assertion, the boundary values and cancellation; the code was committed as c5b7d76 earlier.
