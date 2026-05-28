---
id: RR-08AK
type: review-response
title: Per-GET resolver work tripled; relation predicates are full store scans (S1)
finding: serializeEntityForWire computes verdicts 3x per GET (hiddenProperties, computeFieldAffordances, computeRelationAffordances). Each call re-runs effectiveRoles (HasEdge per role-relation type = full linear scan that clones each match), and re-evaluates every grant predicate (each has_relation/count_relations = another full scan). Cost ~O(3 x grants x relations) per GET plus O(roleRelationTypes x relations) x3. This is the per-row-scan perf cliff CLAUDE.md warns about for a daemon backing the SPA.
severity: significant
resolution: RelationLookup collapsed to a single OutgoingCounts(fromID) map[string]int, cached once per resolve call on bindingContext (lazy, first host-func use). has_relation/count_relations now read the cache instead of scanning per call. storeRelationLookup tallies by type in one ListRelations scan. HasEdge stays a targeted query for local-role resolution. effectiveRoles still calls HasEdge per role-relation type but that's a bounded targeted query, not a full type scan. The dominant per-row scan cliff is removed; remaining 3x verdict computation across serialization is now cheap in-memory map work, documented as a follow-up if it ever matters.
status: addressed
---
