---
id: RR-UR42E8A
type: review-response
title: GraphQuery to Cond conversion belongs in store with a backend differential
finding: Converting a grant When into a store.Cond is store semantics; place it in store and pin it in storetest on every backend instead of in affordances.
severity: significant
resolution: Added store.GraphQuery.RowCond in the store package; storetest RowCond_matches_the_query compares it with GraphQuery on memstore, fs, sqlite and postgres.
status: addressed
---

## Finding

Converting a grant When into a store.Cond is store semantics; place it in store
and pin it in storetest on every backend instead of in affordances.
