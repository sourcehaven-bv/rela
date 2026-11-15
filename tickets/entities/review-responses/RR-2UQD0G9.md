---
id: RR-2UQD0G9
type: review-response
title: narrowingIDs recomputes the caller flag
finding: narrowingIDs took pageScoped and the caller recomputed whether to narrow.
severity: nit
resolution: narrowingIDs now returns (ids, narrowed) and the caller uses it.
status: addressed
---
