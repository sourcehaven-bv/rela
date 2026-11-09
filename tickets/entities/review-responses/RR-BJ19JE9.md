---
id: RR-BJ19JE9
type: review-response
title: Relation type with no end in scope answers an empty range
finding: A relation range whose relation type has no from or no to type in entities always answers count 0, which reads as in sync.
severity: minor
resolution: planReconcileRange refuses it with 400 naming the type and its ends. Pinned in TestReconcile_RefusesBadRequests.
status: addressed
---
