---
id: RR-D49HNSQ
type: review-response
title: PushedFilterMatchesGoFilter does not prove pushdown
finding: A silent fallback to the Go path would pass the equality test.
severity: minor
resolution: TestHandleListEntities_LoweredFilterIsPushed asserts a CountMatched call and no whole-type ListEntities.
status: addressed
---
