---
id: RR-UBXDI
type: review-response
title: 'Asymmetry: properties have properties_unset, but relations have only replacement (no granular remove)'
finding: |-
    Current PATCH has properties_unset for granular property removal. New relations field has only replacement semantics — no 'remove just this edge' verb without re-sending the whole list. For 100-tagged-entity case, client must fetch all 100 IDs, drop one, send 99. With auto-save, this is a fetch-modify-send race window every time.

    May be acceptable for v1 (plan defers Neo4j-style connect/disconnect to v2), but the asymmetry with properties_unset is worth a callout.

    Fix: add Future Work note: 'v2 may add relations_diff: {tagged: {add: [...], remove: [...]}} for granular operations, mirroring properties_unset. v1 is replacement-only because (a) simpler to validate atomically, (b) matches JSON:API §9, (c) form-auto-save use case has the full set in memory anyway. Revisit when we have a use case that doesn't have the full set (large lists, partial updates, optimistic concurrency on individual edges).'
severity: minor
resolution: 'Scope (OUT) entry: ''Granular diff verbs (add/remove/set) — replacement at list level only. v2 may add connect/disconnect-style operations if a use case appears that doesn''t have the full edge set in memory.'' Future work documented.'
status: addressed
---
