---
id: BUG-ZM7SBI
type: bug
title: Nav badge counts leak existence of ACL-hidden entities (ungated enrichNavEntry)
description: 'Nav sidebar badge counts are computed via raw ungated listFromStoreByTypes (enrichNavEntry, app.go:537), so they count entities the principal cannot read under ACL — leaking hidden-entity cardinality. A parallel gated count path (sidebarCounts.countWithFilters) already exists, proving the inconsistency. Same read-ACL ''gate by convention'' class as #1010.'
priority: medium
why1: enrichNavEntry (internal/dataentry/app.go:537) computes item.Count from a raw, ungated listFromStoreByTypes — it counts every entity of the type regardless of the principal's read-ACL.
why2: The navigation count path was written independently of the sidebar count path (sidebarCounts.countWithFilters, api_v1.go:2771), which DOES gate via readGate.ReadQuery/DenyAll. Two parallel count implementations diverged.
why3: 'Read-ACL gating is applied by convention per-call-site rather than enforced structurally — a handler can read raw store data without the gate and nothing flags it (same root pattern as the #1010 sync read-ACL bug).'
status: backlog
---

## Summary

The navigation sidebar badge count (`enrichNavEntry`,
`internal/dataentry/app.go:537`) is computed from a **raw, ungated**
`listFromStoreByTypes(ctx, a.Services(), …)` followed by `applyFilters` +
`len()`. Under an ACL where the principal cannot read some entities of a type,
the badge still counts them — leaking the existence/quantity of hidden entities.

## Evidence

A **parallel, correctly-gated** count path exists for the sidebar:
`sidebarCounts.countWithFilters` (`internal/dataentry/api_v1.go:2771`) resolves
`readGateFromContext(ctx).ReadQuery(ctx, entityType)`, short-circuits `DenyAll →
0`, and scopes via `rqr.Query`. The nav path does none of this. No test asserts
nav counts under ACL.

## Impact

Information disclosure: a user who cannot read entities of type X still sees an
accurate count of them in the nav badge. Low-to-medium severity (count
cardinality, not contents), but it's the same read-ACL "gate by convention"
class as #1010.

## Fix direction

Route `enrichNavEntry`'s count through the gated path — ideally the same
`ReadQuery`-based count the sidebar uses, or the forthcoming `visibleReader`
(TKT-N26KLB M5.0b+). When the App decomposition reaches the nav handler, this
should fall out naturally; filed now so it's tracked rather than lost.

## Discovery

Surfaced during the read-path audit for the `dataentry.App` decomposition
(TKT-N26KLB). The relation-peer-ID exposure considered alongside it is **by
design** (TKT-VQGN: the source-entity gate is the intended boundary) and is NOT
a bug.
