---
id: PLAN-J8NV2X
type: planning-checklist
title: 'Planning: Dashboard cards: configurable width'
started: "2026-10-28"
status: done
completed: "2026-10-28"
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem understood. Every dashboard tile gets one cell of `repeat(auto-fill, minmax(280px, 1fr))`. Matrix, multi-column table, ratio-with-groups and long breakdown cards are cramped. Operators need a per-card width in `data-entry.yaml`.
- [x] Scope defined (below).
- [x] Acceptance criteria documented (below).

**Scope:**

In: a `width:` key on `dashboard.cards[]` and `dashboards.*.cards[]`, for list and query cards alike; load-time validation; grid CSS in `DashboardView.vue`; Configure allowlist lock; data-entry guide.

Out: height (`rowspan`). Cards grow to fit their content and grid rows already stretch to the tallest tile, so a row span adds nothing a width does not. Out: a Configure editor for `width` (backlog ticket). Out: dense packing (see Approach).

**Acceptance Criteria:**

1. `width: wide` makes a tile span two grid columns when the grid has at least two; `width: full` spans the whole row. `width: normal` or no `width` keeps one cell. Test: DashboardView unit test asserts the modifier class per card; manual check in the browser at desktop width.
2. On a grid narrower than two minimum tracks (container < 580px) a `wide` tile takes the whole row (one column), never creating an implicit column or horizontal overflow. On phones (viewport <= 480px) `wide` and `full` tiles take the whole row. Test: manual check at 1280, 700, 400 and 375 px wide.
3. DOM order, reading order and Tab order follow the order of `cards:`. No `grid-auto-flow: dense`. Test: unit test asserts DOM order; manual Tab walk.
4. An invalid value (`width: huge`, `width: 2`) fails config load with `dashboard: card[i] "<title>" has invalid width "<v>" (valid: normal, wide, full)` (wording aligned with the display-mode check in review, RR-6AUIKF). A numeric value adds that width takes a named size, not a column count. Test: table-driven case in `dashboard_cards_validate_test.go`.
5. `/api/v1/_dashboard` and `/api/v1/_config` serve `width` (omitted when unset, so the golden files do not change). Test: config round trip test; existing goldens unchanged.
6. Configure cannot write `width` (locked, listed in `locked`), and editing a card in Configure keeps an existing `width`. Test: configedit guard test passes with the new keys locked.

## Research

- [x] ~~/research~~ (N/A: small change, two options named in the brief)
- [x] ~~Searched for existing libraries~~ (N/A: plain CSS grid)
- [x] Checked codebase for similar patterns: `DashboardView.vue` already makes breakdown and table tiles `grid-column: 1 / -1` on phones; `AccountMenu.vue` uses `@container`; TKT-2SGPJZ locked its new card keys in `internal/configedit/allowlist.go`.
- [x] ~~Reference implementations~~ (N/A: Grafana and Metabase use absolute grid coordinates, which do not fit a flowing card list)
- [x] Reviewed concepts: data-entry-ui, data-entry-server.

**Research Doc:** N/A

**Existing Solutions:** see above.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified (none new)

**Technical Approach:**

Config shape: `width: normal | wide | full` (named sizes), default `normal`.

Why named sizes over `span: N`: the grid is `auto-fill`, so the number of columns depends on the viewport and the sidebar, and the operator cannot know it. CSS cannot clamp `span N` to the tracks present; a span larger than the track count creates implicit columns and overflows. Clamping needs JS measuring or one container query per N. Named sizes map to two rules that are always valid: `full` is `grid-column: 1 / -1`, and `wide` is `span 2`, switched to `1 / -1` by a container query when the grid is too narrow for two tracks. Below 580px the grid has at most two columns in every regime (280px tracks + 20px gap; 200px tracks + 12px gap at <= 768px; fixed 2 on phones), so `1 / -1` there is exactly "two columns or one". Named sizes also leave room for a later `half`/`third` without changing meaning.

Height: out of scope (see Scope).

Packing: no `grid-auto-flow: dense`. Dense fills holes by moving later tiles earlier, so the visual order no longer follows the DOM order; Tab order and screen-reader order would jump around the page (WCAG 1.3.2, 2.4.3). Without dense the largest gap is bounded: one cell before a `wide` tile that does not fit at the end of a row, and the rest of a row before a `full` tile. The operator controls this by ordering cards; the guide says so.

Go: `DashboardCard.Width string` (`yaml:"width,omitempty" json:"width,omitempty"`); validation in `validateDashboardCardSource`'s sibling: a new check run for every card in `validateDashboard`.

SPA: `DashboardCard.width?: 'normal' | 'wide' | 'full'` in `types/config.ts`; `dashboard-card--wide` / `dashboard-card--full` class on the tile; the grid gets a wrapper with `container-type: inline-size` so the rule tracks the grid's own width, not the viewport (the sidebar changes it). The wrapper holds no positioned descendants, so its layout containment changes nothing else.

**Files to modify:**

- internal/dataentryconfig/config.go, dashboard_cards_validate.go, validate.go, dashboard_cards_validate_test.go
- internal/configedit/allowlist.go
- frontend/src/types/config.ts, frontend/src/views/DashboardView.vue, DashboardView.test.ts
- docs-project/entities/guides/GUIDE-data-entry.md (+ regenerated docs/data-entry.md)

## Security Considerations

- [x] Input sources: `data-entry.yaml` only (operator config).
- [x] Validation: allowlist of three values; anything else fails load.
- [x] ~~Security-sensitive operations~~ (N/A: presentation only; no data, ACL or I/O path changes)
- [x] Errors name the card and value from config, which is not secret.

**Input Sources & Validation:** `width` from config, allowlisted at load.

**Security-Sensitive Operations:** none.

## Test Plan

- [x] Test scenarios per criterion (see Acceptance Criteria).
- [x] Edge cases: absent `width`; explicit `normal`; `wide` on a one-column grid; `full` on phones; `wide` on a matrix and a table card; `width:` on a permission-hidden card (irrelevant: the card is not served).
- [x] Negative tests: `width: huge`, `width: 2`, `width: Wide` (case-sensitive, rejected).
- [x] Integration: config load through `loadAndValidate`; DashboardView test through the mounted component; manual check on the demo project.

## Risk Assessment

- [x] Risk: merge conflicts with TKT-5GX50Y and the matrix-axis ticket in `DashboardView.vue`. Mitigation: touch only the grid wrapper, the tile class and CSS.
- [x] ~~Security risks~~ (N/A: presentation only)
- [x] Effort: s

**Risks:** see above.

## Documentation Planning

- [x] User-facing docs: data-entry guide Dashboard section (Card Options row + "Card width" subsection).
- [x] Docs-checklist will be created during review.

**Documentation Impact:**

- [x] docs/data-entry.md - UI changes (via GUIDE-data-entry)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-VR6M07 (phone layout keeps two compact columns; wide/full take the row), RR-XTU5FL (comment ties the 580px clamp to track size and gap), RR-R6EZQ7 (docs: width applies to query cards; ordering avoids gaps).
