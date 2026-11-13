---
id: RR-5PQO65A
type: review-response
title: ClosedBy and Region clone every fork per call
finding: Both go through Forks(), which deep-clones, on every engine move.
severity: minor
reason: At most a few forks per flow and 8 targets each; no measured cost. Index by join when profiling shows it; tracked in TKT-YNQPHPP.
status: deferred
---
