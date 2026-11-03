---
id: RR-VHRV6Q
type: review-response
title: Order comment promised more than the store contract and referred to deleted code
finding: The gatherListPeers comment said each row keeps the order the per-row reads produced. The store only promises a stable implementation-defined order and the comment described code that no longer exists.
severity: minor
resolution: 'The comment now states the real invariant: every backend sorts relations by key whatever the filter so batching does not reorder a cell.'
status: addressed
---
