# Issue 23 — user lookup and time reports
Read-only user lookup and time reporting through CLI and MCP, sharing domain operations.
Implementation only: leave changes unstaged and uncommitted for user review.

## Specs

- S1: "ok implementa busqueda de usuarios y reportes". Deliver user candidates by query and time reports by account ID, project, inclusive dates and explicit IANA timezone; human and JSON CLI output.
- S2: "la busqueda no sera dentro del mcp?". Expose both user lookup and time reporting through read-only MCP tools backed by the same operations as CLI. Preserve existing tools and CLI behavior.
- S3: "si, el pdf no lo genera el mcp lo generará el agente que use el mcp, aunque podriamos agregar despues una generación de pdf para que sea idempotente sin embargo por ahora no lo requerimos". No PDF generation, ticket report, mutations or persistence in this delivery.
- S4: "ok usa solid, no metas ni agrandes los archivos sin sentido preferimos dejar las cosas mantenibles". Separate Jira transport, application policy and CLI/MCP adapters; use small consumer-defined interfaces, avoid unnecessary abstraction and oversized existing files.
- S5: Apply the previously agreed issue-23 reporting design: explicit account IDs (never silently choose among same-name candidates), paginate visible project issues and individual worklogs, filter author and started instant locally, integer-second totals and half-open timezone-aware date interval. Expose bounded scanning and explicit partial totals, warnings and authenticated-user-visible completeness scope. Do not imply organization-wide visibility or a transactional snapshot.

## Tasks

- [x] T1 (S1-S5; route: bounded worker; commit: not authorized): shared reporting contracts and dedicated Jira page readers implemented. RED/GREEN for operations, sanitized HTTP classification and strict JSON decoding; multi-page, DST, cap/budget, overflow, validation-zero-request and in-flight cancellation tests pass, including focused race tests.
- [x] T2 (S1-S5; route: bounded worker + parent recovery; commit: not authorized): CLI users search/report time and read-only MCP tools implemented with shared domain validation and operations. Public tests, original-tool golden compatibility and README updated. Worker failed after writing files; parent recovered and checked them. T2 RED evidence unavailable.
- [x] T3 (S1-S5; route: parent + independent verification; commit: not authorized): parent final build/vet/full tests/lint (0 issues), focused race and diff checks passed (job-29 exit 0). A first verifier found mid-scan permission-status loss; parent reproduced RED and fixed GREEN. A second, complete read-only review (S1-S5 pass, no severe findings, same checks exit 0) confirmed the fix. Advisory: `endpoint_window_exhausted` means the endpoint's visible window was exhausted, not that every Jira user was seen. Unproven: T2 RED evidence (worker failed), live Jira behavior, full-cmd race suite (known pre-existing failures).

## Log

- L1: "ok implementa busqueda de usuarios y reportes"
- L2: User accepted lookup and time report scope; ticket reporting deferred. "si, el pdf no lo genera el mcp lo generará el agente que use el mcp, aunque podriamos agregar despues una generación de pdf para que sea idempotente sin embargo por ahora no lo requerimos"
- L3: "la busqueda no sera dentro del mcp?" supersedes the initial mistaken CLI-only proposal. Both CLI and MCP are included.
- L4: "ok usa solid, no metas ni agrandes los archivos sin sentido preferimos dejar las cosas mantenibles"
- L5: "si dale". Started feature/issue-23-user-time-reports from clean develop using dflow before any repository write. No staging, commit, native freeze, publication or issue closure authorized.
- L6: Official Atlassian user-search docs verified: /user/search filters before offset pagination, returns active matching users within first 1000, requires Browse users and groups; missing permission may produce empty 200 responses. Do not claim an empty result proves no matching person exists. Worklog endpoint returns offset pages under Browse projects, issue-security and worklog-visibility restrictions; embedded worklogs are not a complete reporting source. Sources: https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-user-search/ and https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-worklogs/ .
