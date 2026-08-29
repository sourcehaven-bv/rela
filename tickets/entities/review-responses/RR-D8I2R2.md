---
id: RR-D8I2R2
type: review-response
title: 'icon: none is accepted on kanban columns but the plan gives it no rendering path distinct from an unknown name — reviving the RR-VESDBZ disagreement it claims not to touch'
finding: |-
    Approach §6 says `icon: none` on a kanban column is "accepted-and-equivalent rather than a new behaviour" because `v-if="column.icon"` already handles a missing icon, "once the config layer maps `none` → empty".

    Two problems.

    **1. It depends on the empty-string encoding that RR-4P3WPD rejects.** If `none` is carried through as the literal string `"none"` (the fix recommended there), then `v-if="column.icon"` is TRUE for a column with `icon: none`, and `resolveIcon('none')` returns `DEFAULT_ICON` — a FileText glyph. The author asked for no icon and gets a document icon. The two findings must be resolved together; fixing RR-4P3WPD in isolation silently breaks kanban.

    **2. It re-opens RR-VESDBZ while claiming not to.** That wont-fix finding says kanban's `v-if="column.icon"` makes the `resolveIcon` fallback unreachable, so kanban and sidebar disagree on unknown input. The plan cites it and says this ticket "does not try to" resolve it — but adding a name whose entire purpose is to render nothing means kanban and sidebar must now agree on a THIRD case, not just the two RR-VESDBZ described. Declining to unify them is a decision that needs restating for the new case, not inherited.

    Concretely, the three kanban render sites (`KanbanView.vue:574-580`, `:620-626`, `:637-643`) each need the same `none` guard the sidebar gets, or the surfaces diverge again — which is exactly the complaint RR-VESDBZ logged.
resolution: |-
  Addressed in plan (Approach §6). A shared `hasIcon(name)` helper backs both surfaces, so the three-way decision lives in one place. KanbanView's three render sites switch from `v-if="column.icon"` to `v-if="hasIcon(column.icon)"`. A kanban test row was added — the Test Plan previously had none.
severity: significant
status: addressed
---

## Recommended fix

Introduce one shared helper used by BOTH surfaces, so the three-way decision
exists in exactly one place:

```ts
/** hasIcon reports whether a config-supplied name should render a glyph at all.
 *  `none` is the reserved opt-out; empty/absent means the caller derives its own. */
export function hasIcon(name?: string | null): boolean {
  return !!name && name !== NO_ICON
}
```

Sidebar renders `hasIcon(x) ? resolveIcon(x) : spacer`; kanban renders
`v-if="hasIcon(column.icon)"` with no spacer (a column header has no alignment
column to reserve). That keeps the *semantics* unified — which is RR-VESDBZ's
actual complaint — while allowing the *layout* to differ, which is legitimate.

Export `NO_ICON = 'none'` as a named constant from the same module and use it on
the Go side too (a `const NoIcon = "none"`), so the sentinel is never a bare
string literal in five places.

## Test gap this exposes

The plan's edge-case table says `icon: none` on a kanban column should render no
icon, but the Test Plan has no kanban row at all — every listed test targets the
sidebar or Go. Add a `KanbanView` test asserting a `none` column header renders
no `svg`, or this case ships untested.
