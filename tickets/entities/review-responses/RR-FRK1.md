---
id: RR-FRK1
type: review-response
title: ?include=* does O(N) per-id Visible probes — hub-entity perf cliff
finding: 'resolveV1Includes iterates ALL outgoing AND incoming relations. An entity with 50 neighbours triggers 50 Visible calls = 50 GraphCount round-trips in pgstore. Hub entities (features with many tickets, projects with many entities) hit this on every GET. Fix: batch per neighbour-type. Collect candidate IDs grouped by type, then for each type: call ReadQuery(type) once — AllowAll → accept all of that type; DenyAll → drop all; Query → ONE GraphQuery(WhereIDs=[id1,id2,...]) per type and intersect IDs in Go. Cuts 50 round-trips to N-types (typically 2-3). Plan should pin ''include filter uses one GraphQuery per neighbour-type, not per neighbour''; the existing AC4 covers correctness but not perf shape.'
severity: significant
resolution: filterVisibleIncludes groups candidates by type then makes ONE PermitsReadMany call per type. Worst case collapses from O(N-neighbors) to O(N-distinct-types). Implementation post-rework uses readGate.PermitsReadMany so the gate (not the handler) executes the predicate.
status: addressed
---
