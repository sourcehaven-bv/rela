---
finding: Context parameter is ignored in handleSQLQuery. sqldb.Query creates its own context.Background() that cannot be cancelled, risking resource exhaustion on expensive queries.
id: RR-k5mp
resolution: Added context parameter to Query() function and propagated from MCP handler and CLI command
severity: critical
status: addressed
title: No query timeout/context propagation
type: review-response
---
