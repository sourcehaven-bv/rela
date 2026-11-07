---
id: RR-G3BWHOO
type: review-response
title: WithFlowRuns refuses any schema with a conversion from a faceless type
finding: validateConversionRelations demanded a mapping for _carries/_produced, which a schema cannot name, and planRelations marked an incoming managed edge as unmapped.
severity: critical
resolution: The completeness check skips managed relations. planManagedRelation keeps a managed edge when the target type may still be its endpoint and drops it otherwise. This supersedes the "conversion is refused" line of RR-BLECTJJ. Pinned by TestWithFlowRuns_Conversions and TestConvert_KeepsAFlowRunsEdge.
status: addressed
---
