---
id: RR-8YAE1J
type: review-response
title: Relation counts scan every edge per schema call
finding: CountRelations loads all edges of a type and gates their endpoints; the schema overview does this per relation type
severity: significant
resolution: Deferred to TKT-OJ1UYJ. Added TestScriptReader_CountRelationsBudget; it pins the scan to a fixed number of store calls at 10 and 50 edges.
reason: 'Relations have no store pushdown for the endpoint gate. Building one is a join over the composed ACL query per endpoint type: a separate change of size m. The scan is batched (one edge query and one header batch) and only the remote MCP schema overview and summary prompt use it. So the cost is linear memory and not per-row queries.'
status: deferred
---
