---
id: RR-7M2ZOHQ
type: review-response
title: Batch-sized pages have no memory cap
finding: A page sized to the id batch has no upper bound on memory (security review).
severity: minor
resolution: Full entities keep the fixed page (bodyPage). Header and relation rows are small and bounded by the ids the caller already holds, which pagePlan documents.
status: addressed
---
