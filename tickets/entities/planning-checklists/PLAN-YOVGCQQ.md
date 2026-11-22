---
id: PLAN-YOVGCQQ
type: planning-checklist
title: 'Planning: Sankey view for data-entry (sankeys: config, layout, view)'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: a `sankeys:` config block in `data-entry.yaml`, a nav entry, a `_sankeys/{id}` endpoint, a hand-written layout library and Vue view (no d3), a detail panel on node click, and evidence "pips" fetched with one batched relation query per pip type. Out: sankey tabs on pages, node weight from a property, live refresh, static export.

**Acceptance Criteria:**
1. A configured sankey renders nodes and weighted ribbons. Tested in `sankey_handler_test.go` and `SankeyView.test.ts`.
2. Hidden entities never appear, are not counted in `truncated`, and do not appear in pip counts. Tested by the ACL cases in `sankey_handler_test.go`.
3. Clicking a node or pip opens the detail panel and scrolls the node into view. Tested in `SankeyView.test.ts` and checked in the browser.
4. Pip lookups do not grow with the number of nodes. Tested by `TestSankey_PipsRelationQueriesDoNotGrowWithNodes`.
5. Bad config fails at load with a specific error. Tested in `sankey_test.go`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: a sub-agent compared d3-sankey and alternatives in the design discussion)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
d3-sankey was considered and rejected: our own layout (Fenwick-tree crossing count, barycentre sweeps, Bézier ribbons) avoids a dependency and lets Vue own the rendering. The gantt endpoint (`gantt_handler.go`) is the template for config, handler, wire types and view. The prototype `2026-11-17-atlas-sankey.html` inspired the look.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
Config in `internal/dataentryconfig/sankey.go`, handler in `internal/dataentry/sankey_handler.go` and `sankey_pips.go`, wire types in `apiwire/v1`, SPA view `SankeyView.vue` with `sankeyLayout.ts`. The order is row-gate, redact once, then fold, then cap. Pips are read in one gated read plus one batched relation query per pip type.

**Files to modify:**
`internal/dataentryconfig/*`, `internal/dataentry/*`, `internal/apiwire/v1/responses.go`, `frontend/src/{api,views,utils,router,stores,types,i18n}`, `docs-project/entities/guides/GUIDE-data-entry.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
Sources are the `sankeys:` config (validated at load against the metamodel: types, relations, joins, `max` bounds, `color_by`) and the `where` and id query parameters (parsed by the existing predicate engine and ref parser).

**Security-Sensitive Operations:**
Reading entities and relations. Every read goes through the visibility gate; pip peers are gated before title or tone is read, and `+n` counts only gated peers so it is not an existence oracle. The response is per-principal and is not cached across principals.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
See the acceptance criteria. Go handler tests run against the real handler with a memory store; the SPA has component and layout tests.

**Edge Cases:**
Empty graph, cycles, faced (content-scoped) nodes, truncation at the cap, pips over `max`, a narrow area with the panel open.

**Negative Tests:**
Unknown relation or type, a pip type equal to a node type, a relation that does not join, `max` above 50, a hidden node or pip.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
Layout quality on dense graphs (capped by node limit and sweeps). Per-row lookups for pips (one batched query per type, pinned by a call-count test). Plimsoll field limits (pins raised per key).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - UI changes (generated from `GUIDE-data-entry.md`: new "Sankeys" section)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was agreed in discussion and the work is already built; `/code-review` covers it in the review phase)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
