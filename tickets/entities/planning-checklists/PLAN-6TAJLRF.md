---
id: PLAN-6TAJLRF
type: planning-checklist
title: 'Planning: Dashboard: one card per row on phones'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem understood. At <= 480px the dashboard grid has two tracks and only breakdown and table cards span the row, so count cards sit two per row. Jeroen decided phones show one card per row, whatever its `width`.
- [x] Scope defined (below).
- [x] Acceptance criteria documented (below).

**Scope:**

In: the <= 480px rule in `DashboardView.vue`, the card-width comment, the phone
sentence in the `width` guide, the e2e width spec.

Out: the 481-768px layout; the file's existing prettier and max-lines warnings.

**Acceptance Criteria:**

1. At a 375px viewport the grid has one track, and every card (normal, wide, full) takes the whole row and starts at the same x. Test: `e2e/tests/dashboard-card-width.spec.ts`.
2. No horizontal overflow at 375px, including with table cards. Test: the same spec asserts `overflows === false`; `minmax(0, 1fr)` keeps the track from growing past the viewport.
3. Layout above 480px is unchanged. Test: the existing 1440, 900 and 700px cases still pass.

## Research

- [x] ~~/research~~ (N/A: one CSS rule)
- [x] ~~Searched for existing libraries~~ (N/A: plain CSS grid)
- [x] Checked codebase: the 480px rule and the `:has(...)` full-row rules in `DashboardView.vue`; the `wide` container query switches to `1 / -1` below 580px, so it already fits one track.
- [x] ~~Reference implementations~~ (N/A: one CSS rule)
- [x] Reviewed concepts: data-entry-ui; TKT-H21TYS set up card widths.

**Research Doc:** N/A

**Existing Solutions:** see above.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified (none)

**Technical Approach:**

Replace `repeat(2, minmax(0, 1fr))` with `minmax(0, 1fr)` at <= 480px and remove
the two `:has(...)` rules, which become redundant. Alternative: keep two tracks
and make count cards span the row. Rejected: it adds a rule per card kind
instead of removing two.

**Files to modify:**

- frontend/src/views/DashboardView.vue
- e2e/tests/dashboard-card-width.spec.ts
- docs-project/entities/guides/GUIDE-data-entry.md and docs/data-entry.md

## Security Considerations

- [x] Input sources: none new.
- [x] ~~Input validation~~ (N/A: CSS only, no input)
- [x] ~~Security-sensitive operations~~ (N/A: presentation only)
- [x] ~~Error handling~~ (N/A: no errors added)

**Input Sources & Validation:** N/A

**Security-Sensitive Operations:** N/A

## Test Plan

- [x] Test scenarios documented (see acceptance criteria)
- [x] Edge cases: a wide or full card at 375px; a table card (min-content wider than the viewport).
- [x] ~~Negative test cases~~ (N/A: no input to reject)
- [x] Integration test: Playwright measures the layout, because jsdom has none.

**Test Scenarios:** see acceptance criteria.

**Edge Cases:** see above.

**Negative Tests:** N/A

## Risk Assessment

- [x] Risk: phones show fewer count cards per screen. Accepted by decision.
- [x] ~~Security risks~~ (N/A: CSS only)
- [x] Effort: s

**Risks:** see above.

## Documentation Planning

- [x] User-facing docs: the phone sentence under "Card width" in the data-entry guide.
- [x] Docs-checklist will be created for this enhancement.

**Documentation Impact:** docs/data-entry.md (from GUIDE-data-entry).

## Design Review

- [x] Design review done inline: no critical or significant findings.
- [x] ~~All critical/significant findings addressed~~ (N/A: none found)

**Design Review Findings:** none
