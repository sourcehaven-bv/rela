---
id: RR-FC1C
type: review-response
title: 'S2 + S3: grouped-cards branch + string-mirror sync'
finding: |
  - S2: Plan's "if grouped cards ship" case is speculative. Confirmed by checking `internal/dataentry/sections.go`: GroupBy only runs for table display. The cards/list/content branches never produce `Groups`. IHC7D's own commit notes call grouped-cards "currently dormant."
  - S3: Plan says "optionally update `fields[i].values` string mirror — mostly cosmetic since `_props` is the source of truth." This is wrong: today's display-mode cell renders from `row.field.values` (via `fieldRowsFor`). If a row's verdict flips from writable to non-writable mid-session and the string mirror is stale, the user sees stale data in display mode.
severity: significant
status: addressed
resolution: |
  - S2: PLAN's grouped-cards edge case dropped. Comment in code documents: "Grouped cards have no backend producer today; when added, this path needs parallel wiring." No code shipped for the speculative case.
  - S3: PLAN AC 5 amended: `applyPropertyToRow` updates BOTH `_props[prop]` AND the matching `fields[i].values` string mirror via `propertyToStrings` (the existing display-stringifier already exported from backend; the frontend equivalent is the widget's display-mode render that consumes `model-value`). Easier: the row's display mode reads from `_props` first, falling back to `fields[i].values` for legacy. Pick (b) — read from `_props` first — so the verdict-flip stale-mirror bug can't surface. Update `fieldRowsFor` / the row display template to prefer `ent._props?.[f.property]` over `f.values` when `_props` is present. One small change, eliminates the stale-mirror class of bugs.
---
