---
id: RR-9P8JYE
type: review-response
title: storeutil.MatchRelation allocated the EntityIDs set per relation row
finding: The EntityIDs branch built a lookup map inside MatchRelation; fsstore and memstore call it once per relation, so an N-row scan with an M-element batch was N allocations of size M.
severity: significant
resolution: storeutil.NewRelationMatcher compiles the query (and its set) once into a predicate; MatchRelation delegates to it; every fsstore and memstore loop (ListRelations, ListRelationsPage, CountRelations) builds the matcher once per call.
status: addressed
---
