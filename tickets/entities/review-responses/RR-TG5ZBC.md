---
id: RR-TG5ZBC
type: review-response
title: Face-gate acceptance is not mechanically enforced
finding: The Stage 0 guard does not forbid GetEntityState with a face or GetEntityAt or single-id ListEntities so handlers can still bypass the resolver
severity: significant
resolution: 'Design section 8.5 (PR 3 and PR 5): the Stage 0 guard is extended to forbid GetEntityState with any face and GetEntityAt and single-ID ListEntities/ListEntityHeaders in dataentry/mcp/lua; the allowlist shrinks to write-prep entries with reasons.'
status: addressed
---

**Where:** ticket acceptance "A handler cannot obtain a row without the face
gate having run"; design section 5.

The Stage 0 guard (`internal/archguard/zeroface_test.go`) forbids
`store.GetEntity`, `GetEntityState(ctx, id, "")`, `entityReader.getEntity` and
`bareEntityID`. It does not forbid `GetEntityState` with a non-empty face,
`GetEntityAt`, or a single-id `ListEntities`. Those are exactly the raw reads
the face-gate pairs use today (`api_v1.go:1111`, `viewworld.go:152`,
`actions.go:232`). After Stage 1 a new handler can still call them without the
resolver, and nothing fails. The acceptance criterion is not mechanically
checked.
