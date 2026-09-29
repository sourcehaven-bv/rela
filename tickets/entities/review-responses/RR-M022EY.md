---
id: RR-M022EY
type: review-response
title: Cardinality analysis runs one CountRelations query per subject
finding: internal/dataentry/analyze.go and internal/schema/cardinality.go count per subject, and per face row for content-scoped outgoing bounds. Pre-existing, amplified by per-face bounds.
severity: significant
resolution: 'TKT-5LW875: schema.CheckCardinality reads each relation''s edges with one ListRelationsStrict query and each subject type''s headers once, then counts in memory. Data-entry calls the same checker. TestCheckCardinality_ReadBudget pins equal reads at 10 and 50 subjects.'
reason: Batching changes which reader counts (a gated edge list versus a raw count on MCP), which is the same decision as the security finding on raw counts. Both go to TKT-5LW875 so they are decided once.
status: addressed
---
