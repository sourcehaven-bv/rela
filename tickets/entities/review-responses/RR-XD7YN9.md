---
id: RR-XD7YN9
type: review-response
title: MCP neighbor lookup costs more reads per edge
finding: convert.go neighbor does Resolve per edge and Family for a bare id that misses. That is a per-row lookup on show_entity; the edges already come from a gated ListRelations.
severity: significant
reason: The per-edge read predates this PR (one GetEntity per edge). The extra Family read only runs when Resolve misses (a faced neighbor by bare id). The second check is deliberate defense in depth (RR-CFFL52; TestACL_BuildStoreRelations_WithholdsUnreadableEdge). Batching needs a gated batch-title read on GraphReader; an mcp ListEntityHeaders with IDs is forbidden by the 8.5 guard. Belongs with the collection-read work for MCP.
status: deferred
---
