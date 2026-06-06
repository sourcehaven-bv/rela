---
id: RR-R0NR
type: review-response
title: WhereIDs total semantics unpinned — backends could silently diverge
finding: 'TKT-VQGN adds `WhereIDs []string` to store.GraphQuery but never pins what GraphCount.total returns when WhereIDs is set. pgstore.total today is `SELECT count(*) FROM entities WHERE type = $1` ignoring predicates; naive.total is `len(candidates)` over all entities of type. Both would keep total = full type cardinality if WhereIDs is implemented as a result-set predicate, but a future implementer reading the field name could fold it into total, making total == matched == 1 for every Visible call. Fix: pin in GraphQuery godoc that WhereIDs is a PREDICATE (constrains matched), NOT a candidate-set restriction; total IGNORES WhereIDs. Add storetest case: 2-entity store, WhereIDs=[nonexistent] returns total=2, matched=0.'
severity: critical
status: open
---
