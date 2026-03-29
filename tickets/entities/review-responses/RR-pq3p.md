---
finding: No limit on returned rows. SELECT * FROM large_table could cause memory exhaustion through amplification (each row becomes a map with multiple allocations).
id: RR-pq3p
resolution: Added MaxRows=10000 limit with Truncated flag in QueryResult. MCP handler and CLI show truncation warning.
severity: significant
status: addressed
title: No row count limit
type: review-response
---
