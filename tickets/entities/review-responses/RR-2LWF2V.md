---
id: RR-2LWF2V
type: review-response
title: EXPLAIN helpers copy production page SQL assembly
finding: BuildEntityListSQLForTest repeated the fetch++ logic and sqlite ExplainEntityPage repeated the LIMIT logic and ignored the cursor.
severity: minor
resolution: Extracted entityPageSQL(q) in both stores. ListEntitiesPage and the test helpers call it so EXPLAIN reads the SQL production sends including the cursor.
status: addressed
---
