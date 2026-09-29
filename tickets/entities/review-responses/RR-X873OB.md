---
id: RR-X873OB
type: review-response
title: '[security] MCP cardinality returns raw gate error text'
finding: '[security] handleAnalyzeCardinality returns err.Error() to the client, and a gate fault now reaches that path. Data-entry logs the cause and returns a generic message.'
severity: minor
reason: 'Pre-existing pattern: about 46 MCP handlers return err.Error(). Changing one tool would leave the surface inconsistent; it belongs in one pass over the MCP error contract, not in this ticket.'
status: deferred
---
