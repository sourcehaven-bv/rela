---
id: PLAN-YJWGR40
type: planning-checklist
title: 'Planning: Dashboard cards render values like the rest of the app'
started: "2026-11-01"
completed: "2026-11-01"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: breakdown and table cards (query and list source) render enum values
with authored labels and `styles:` colours, currency integers as amounts and
dates formatted, reusing the list view's dense cell widgets and Badge; breakdown
groups in declared enum order; docs fix for map-form `sort:` examples. The plan
was approved by the user up front.

Out of scope: card width (done in TKT-H21TYS, `width: normal|wide|full`);
single-map `sort:` in the loader (no `sort:` accepts it; `rela migrate` converts
it); relation columns in dashboard tables; server changes (list cards already
get labels and enum order from the server).

**Acceptance Criteria:**
1. A breakdown on an enum shows the authored label as a badge with the style colour, and the bar in that colour. Test: Vitest (query card and list card), e2e on a fixture dashboard.
2. Breakdown groups follow the declared enum order; values outside the enum follow by count, unset last. Test: Vitest with counts that disagree with enum order.
3. A table card renders an enum cell as a labelled, styled badge, a `display: {currency}` integer as an amount and a date formatted. Test: Vitest (query and list card), e2e with locale nl-NL.
4. Detail links and hidden-value cells keep working. Test: existing Vitest stays green.
5. Docs show `sort:` as a list everywhere. Test: grep for map-form sort in docs; generated docs match.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach fixed by the approved plan)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: the app already has the renderers)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** EntityList renders cells with `densePropertyRoutingHint`
+ `defaultRegistry.resolveDense` and pre-formats passthrough kinds with
`formatCellValue` (dates, currency via `formatDisplayNumber`). `Badge.vue`
resolves label and style from (property, entityType). `styleClassFor` /
`styleSwatch` give a style's colour. The server's `groupCounts`
(dashboard_fold.go) already orders list-card groups by declared enum order, then
count, unset last.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- New `frontend/src/utils/denseCell.ts`: `resolveDenseCell(value, property, entityType, tz)` returns the widget, property name and model value for a dense cell, or undefined for an empty value or unknown property; caches the widget per (type, property). Used by the dashboard table.
- DashboardView breakdown: each group carries its raw value and the entity type; enum groups render as `Badge` (label and colour) and the bar takes the style colour (`styleBarColor`, new in styleColors.ts). Query-card grouping orders like `groupCounts`.
- DashboardView table: cells render the resolved widget, with the plain string as fallback. List-card rows carry `type`, so the same path serves both.
- Docs: GUIDE-data-entry dashboard and list examples use list-form `sort:`; card options table says `sort` is a list; display-mode text documents order and cell rendering; regenerate docs/data-entry.md.

Alternatives: moving query-card grouping to the server (rejected: query cards
run client side by design); duplicating EntityList's `columnWidgets` in
DashboardView (rejected: one shared helper); count-desc order (rejected: a
pipeline reads in stage order, and list cards already use enum order).

**Files to modify:** frontend/src/views/DashboardView.vue,
DashboardView.test.ts, frontend/src/utils/denseCell.ts (+test),
frontend/src/utils/styleColors.ts, e2e/tests/dashboard-card-rendering.spec.ts,
e2e/pages/dashboard.page.ts, docs-project/entities/guides/GUIDE-data-entry.md,
docs/data-entry.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** operator config (already validated at load);
entity values the reader already received from gated endpoints. Labels are
interpolated text, never HTML. Style classes go through `styleSwatch`, which
only accepts classes the server can emit.

**Security-Sensitive Operations:** none. Rendering only; no new data is read.
Redacted cells keep the hidden marker.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria. Integration: e2e spec with a
named dashboard appended to the fixture project, locale nl-NL.

**Edge Cases:** enum value outside the declared values (raw value, after
declared ones); unset value (unknown label, last); non-enum group_by (plain
text, count order); value without a style (accent bar, grey badge); empty cell
(blank); redacted cell (hidden marker); mixed-type query (per-row type).

**Negative Tests:** unknown property in a column (plain string fallback);
non-numeric value in a currency column (plain value).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** locale-dependent amounts and dates: tests pin locale. Badges in a
narrow breakdown label column could overflow: labelled badges already truncate
at 24ch. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-data-entry dashboard section and list example
(sort shape, breakdown order, table cell rendering); regenerated
docs/data-entry.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the user reviewed and approved the plan before the ticket was created)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
