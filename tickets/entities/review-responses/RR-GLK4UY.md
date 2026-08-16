---
id: RR-GLK4UY
type: review-response
title: Display-value staleness in mapFieldsToProperties promoted from unreachable to common by the render default
finding: '`mapFieldsToProperties` (EntityDetail.vue:423) read `field.values` — the server-side display-stringified mirror — while `handlePropertyApplied` (:519) rewrites only `entry.properties` via `applyPropertyToEntry`. Before TKT-HOIX1 the entry display path (`PropertyDisplay`, EntityDetail.vue:879) was reachable only when the ACL denied EVERY field in a section, so nothing could edit those values and the mirror could never go stale. With `render` defaulting to display, a display-rendered section is now the COMMON case and can sit alongside a `render: input` section that PATCHes the same property — at which point the display section shows last-loadView''s string indefinitely. The ticket had scoped this out as pre-existing (RR-8EISWO), but that deferral rested on a reachability argument the default flip invalidates. Silent wrong-data-on-screen.'
severity: critical
resolution: 'Added `entryDisplayValue()` in EntityDetail.vue, the entry-section analogue of the existing `rowDisplayValue` (which already solved this for cards/list rows under RR-FC1C): prefer live `entry.properties[field.property]` when present, fall back to `field.values`. Pinned by e2e `a display section reflects the current server value` in view-section-render-mode.spec.ts.'
status: addressed
---

Verified in source before accepting:

- `EntityDetail.vue:440` — `value: field.values ?? []` (server mirror, never updated post-PATCH)
- `EntityDetail.vue:519-526` — `handlePropertyApplied` → `applyPropertyToEntry`, which rewrites
`entry.properties` only
- `EntityDetail.vue:575-580` — `rowDisplayValue` already prefers `_props` for the row path,
with a comment naming this exact bug class (RR-FC1C)

Reachability confirmed as *newly* common rather than *newly* introduced: the
in-repo `tickets/data-entry.yaml` has views with multiple entry-source sections
(`idea`, `future-concept`), though their property lists are currently disjoint
and all migrated to `input`, so no live config hits it today. An unmigrated
config with overlapping properties across two entry sections would.

The fix mirrors the row path exactly rather than inventing a second mechanism.
