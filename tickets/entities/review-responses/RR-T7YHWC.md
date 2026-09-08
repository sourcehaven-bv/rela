---
id: RR-T7YHWC
type: review-response
title: Nested-anchor risk from a URL-typed first column is not reachable
finding: |-
    Raised: if the first column routed to a widget that emits its own `<a>`, the row
    anchor would wrap it, producing invalid nested-anchor HTML that browsers unnest
    unpredictably.
severity: significant
resolution: |-
    NOT APPLICABLE — investigated and rejected rather than guarded.

    `WidgetHintKind` (frontend/src/widgets/types.ts:87) has exactly nine members:
    text, text-list, enum, enum-list, boolean, date, datetime, integer, rrule. None
    emits an anchor. `FileWidget.vue` is the only widget in the tree that renders an
    `<a>`, and it is unreachable from this path — `viewRouting.ts:148` carries a
    comment stating that `WidgetHintKind` has no 'file' member so the hint path
    cannot reach FileWidget.

    Adding a guard for an unreachable case would be dead code that reads as though
    it were protecting something. If a link-emitting dense widget is ever added, the
    routing table is where the guard belongs, not the cell wrapper.
status: addressed
---

## Resolution

NOT APPLICABLE — investigated and rejected rather than guarded.

`WidgetHintKind` (frontend/src/widgets/types.ts:87) has exactly nine members:
text, text-list, enum, enum-list, boolean, date, datetime, integer, rrule. None
emits an anchor. `FileWidget.vue` is the only widget in the tree that renders an
`<a>`, and it is unreachable from this path — `viewRouting.ts:148` carries a
comment stating that `WidgetHintKind` has no 'file' member so the hint path
cannot reach FileWidget.

Adding a guard for an unreachable case would be dead code that reads as though
it were protecting something. If a link-emitting dense widget is ever added, the
routing table is where the guard belongs, not the cell wrapper.
