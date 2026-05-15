---
id: REV-ZN2W
type: review-checklist
title: 'Review: Migrate MCP server to wire its own services (off Workspace)'
status: done
---

## Code Review

- [x] cranky-code-reviewer agent run on the diff
- [x] All 3 critical findings addressed in-PR
- [x] 5 of 7 significant findings addressed (1 deferred with rationale; 1 covered by #2)
- [x] 4 minor findings addressed (rename, slog.Warn, compile-time assertion)
- [x] Leverage opportunities deferred to follow-up
- [x] Tests pass under `-race`
- [x] `just ci` passes end-to-end

**Summary:** see IMPL-WKZZ for the full disposition table.

**Code Review Summary:**

Cranky review caught three real bugs:
1. `errSearcher` duplicated in two places (`workspace/services.go` + `cli/mcp_wiring.go`). Fixed by lifting to `search.ErrSearcher`.
2. Backfill swallowed errors silently — the exact lesson from TKT-LCTG. Fixed by mirroring the workspace pattern (collect list+index errors, return summary, caller slog.Warn).
3. `runMCPServer` flattened every wiring failure to "no project found" — a metamodel parse error told the operator to `rela init`. Fixed by distinguishing `ErrNoProject` from other diagnostics.

Plus a footgun in the test fixture (`newTestServices` silently bypassed cascade
if metamodel declared automations — fixed by mirroring production wiring) and a
missing test surface for the wiring helper (8 unit tests added in
`mcp_wiring_test.go`).

Net result: TKT-KWAX migration is observably correct (existing tests + 8 new
tests pass under race), with logging in place to make incomplete states visible.
