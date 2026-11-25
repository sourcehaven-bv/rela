---
id: RR-E6L5ME6
type: review-response
title: Cost on the write path
finding: Binding system:flow on every watched update, List under the write lock, synchronous settle.
severity: minor
resolution: Bind and role lookup only when the subject has open runs or the type has predicate starts. Sharing the List with WriteRecheck and the synchronous settle are left as is (same cost model as create starts).
status: addressed
---
