---
id: RR-K72ML0
type: review-response
title: Per-principal member-of walk has no cross-principal caching (store-traffic multiplier)
finding: Memoization is per-Request only (Globals cached on the Request; a Request is bound to one principal). Enumerating N principals = N fresh member-of BFS walks + N ancestor walks, each HasEdge probe a store GetRelation. All walks are bounded (visited-set + depthCap=5, no runaway), but store traffic is linear in principals with zero sharing, and there's no batch amortization for the attribution path (only reads have PermitsReadMany). Acceptable for single-entity who-can at typical principal counts; note it as a known cost and a reverse-index optimization target for the later map command.
severity: minor
resolution: Documented as a known, bounded cost (N member-of walks, depthCap=5, no runaway) acceptable for single-entity who-can. A reverse principal->entity index is explicitly deferred to the later map command, listed under deferred items. No change needed for this ticket.
status: addressed
---
