---
id: RR-7Q9Z
type: review-response
title: handleV1EntityRelations ignores request context
finding: api_v1.go:710 calls a.outgoingRelations(entityID) which uses context.Background(). The handler has r.Context() right there. Pre-existing concern but widened by this PR's per-group sort cost. Use outgoingRelationsCtx (the existing helper) instead.
severity: significant
resolution: handleV1EntityRelations now uses r.Context() through outgoingRelationsCtx and a newly-added incomingRelationsCtx. Iterator errors surface as 500 instead of being silently dropped.
status: addressed
---
