---
id: RR-SYFX1B
type: review-response
title: entityTarget() called 2x per row per render and is not cheap
finding: EntityList.vue called entityTarget(entity) twice per row per render (once in the v-else-if, once in the :to), and the function is not cheap — it iterates all of route.query, maps over sortSpecs, runs listConfig.columns.find(), and allocates a fresh object each call. At 25 rows that is 50 invocations per render, re-running on every reactive tick including hover-driven selectedIndex changes. The codebase already memoises resolveCell per column for exactly this reason (RR-UD2A).
severity: significant
resolution: Added a rowTargets computed Map (entity id -> RouteLocationRaw) alongside the existing columnWidgets memo, citing RR-SYFX1B and the same RR-UD2A rationale. All four template references and navigateToEntity now read the map, so entityTarget runs once per entity per render instead of twice per row on every reactive tick.
status: addressed
---

**Finding (S1, significant).** `EntityList.vue` calls `entityTarget(entity)`
twice per row per render (once in `v-else-if`, once in `:to`), and the function
is not cheap: it iterates all of `route.query`, maps over `sortSpecs`, runs
`listConfig.columns.find()`, and allocates a fresh object each call.

At 25 rows that is 50 invocations per render, re-running on every reactive tick
— including `selectedIndex` changes driven by hover/keyboard.

This codebase already knows better: `resolveCell` carries a comment (`:625`)
saying it is memoised per column "rather than per cell (RR-UD2A -- the same
reason PropertyDisplay precomputes `rows`)". The same reasoning applies here,
and this change also added two more `resolveCell` calls inside the new
RouterLink branch.

**Resolution:** precompute a `computed` map of entity id → target. The query
half is identical for every row in the list — only `path` varies — so most of
that work is per-list, not per-row.
