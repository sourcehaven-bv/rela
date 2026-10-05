---
id: RR-CZMJ4T
type: review-response
title: LinearSearch admits the whole type before matching text
finding: listAdmitted runs MatchingFaces over every id of each scoped type per query.
severity: minor
reason: LinearSearch backs the memorybackend tag only (tests and experiments); no production tier uses it. The cost is one query per type, not per row.
status: wont-fix
---
