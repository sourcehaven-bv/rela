---
id: RR-TEA3NA
type: review-response
title: MCP still does a pre-read that PatchEntity repeats
finding: 'internal/mcp/tools_entity.go:198-215 still calls st.GetEntity(ctx, id) solely to obtain e.Type for validatePropertyNames, then PatchEntity does its own raw read. Two reads where one might do, and the window between them is racy: if the entity is deleted in between, the caller gets PatchEntity''s ErrEntityNotFound rather than the handler''s ''entity not found: ''+id — different text for the same condition on the same code path. Not a correctness bug (the second read is authoritative; the first feeds only an advisory NAME check), but it is exactly the duplicated read this ticket set out to consolidate.'
severity: minor
reason: |-
    DEFERRED, not dismissed — removing it is a behaviour change needing its own decision, and doing it here would exceed the ticket's stated scope.

    The pre-read exists so validatePropertyNames can reject unknown property names with a good message BEFORE dispatching. Eliminating it means one of: (a) dropping early name validation and letting the manager's metamodel validation own it — changes MCP's error text and its 400-vs-warning shape, which AC6 explicitly promised to leave unchanged; (b) having PatchEntity surface the resolved type so the caller can validate after the fact — widens the primitive's return contract for one consumer's convenience; (c) moving name validation into the manager — a real improvement but a cross-cutting change affecting every writer.

    The divergent error text on the racy path is the only user-visible symptom, and both outcomes are a truthful 'not found'. Note MCP is slated for replacement by a remote-oriented server, which is when (c) becomes worth doing properly.

    Mitigated meanwhile: the misleading 'one read' claim in updateCore's godoc was corrected (RR-E812RW) so the duplication is documented rather than contradicted.
status: deferred
---
