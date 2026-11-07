---
id: RR-ANDDXA0
type: review-response
title: openBackend leaks the store listener on late failures
finding: After pgstore.Open succeeds, the pgmigstate/pgcomments/pgschedstate error branches closed only the pool, not the store.
severity: minor
resolution: Those branches call closeBackend(st, poolCloser).
status: addressed
---
