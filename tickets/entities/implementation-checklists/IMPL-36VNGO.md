---
id: IMPL-36VNGO
type: implementation-checklist
title: 'Implementation: Dashboard cards: configurable width'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests: `TestValidateDashboardCardWidth` (unset, normal, wide, full, unknown, case, column count) and `TestValidateDashboardCardWidth_NamedDashboard`; DashboardView "card width" tests (class per width in config order; source pins no dense packing and the container clamp).
- [x] Integration: config loads through `loadAndValidate` (the full YAML parse + validate path); DashboardView mounted with a seeded store; configedit guard test classifies the new keys.
- [x] Happy path implemented: `DashboardCard.Width`, load validation, `dashboard-card--wide` / `--full` classes, grid CSS, allowlist lock, guide.
- [x] Edge cases from planning: unset and explicit normal render one cell; a wide tile on a one- or two-track grid takes the whole row; unknown values fail load; a hand-crafted unknown value in the SPA renders as normal.
- [x] Errors surfaced: invalid width fails config load with a message naming the dashboard, card index, title and value.

## Test Quality

- [x] ~~Fixture builders~~ (N/A for config YAML; the SPA test uses the file's existing `card()` builder)
- [x] No hardcoded values where an object is in scope
- [x] Only specifying values that matter (title and width)
- [x] ~~Interpolated values from objects~~ (N/A: expected messages are the contract under test)
- [x] ~~Property comparisons use original object~~ (N/A: see above)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

Copy of the grc-gemeente demo in /tmp with the risk matrix and grouped ratio card `wide` and the incidents table `full`, served by `rela-server-sqlite` built from this branch on port 49301; `rela validate` passes. Playwright measured tile geometry at five viewports:

- 1440px (grid 1115px, 3 tracks): wide tiles 737px (two tracks), full 1115px, normal 358px; no horizontal overflow.
- 900px (grid 575px with sidebar, 1 track): every tile 575px; no overflow (the container clamp, not a viewport query, decided this).
- 700px (sidebar collapsed, grid 668px, 3 tracks of 215px): wide 441px, full 668px.
- 400px and 375px (phone, 2 tracks): wide and full take the whole row; count tiles stay two-up; no overflow.

DOM order equals config order at every width (no dense packing), so Tab order follows the config. Invalid width: `width: 2` and `width: huge` fail load with the planned messages (unit tests).

## Quality

- [x] Follows project patterns: the validation sits beside the other card checks; the allowlist lock mirrors TKT-2SGPJZ; the guide section follows the card sections.
- [x] DRY: allowed widths are one slice used for the check and the message.
- [x] No security issues: presentation only; config allowlisted.
- [x] No silent failures.
- [x] No debug code left behind.
