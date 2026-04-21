---
id: RR-JWDHH
type: review-response
title: outgoingRelations uses context.Background() instead of request context
finding: reconcileOutgoingRelations receives a request ctx and passes it to writes, but the upfront read goes through a.outgoingRelations which hardcodes context.Background(). Request cancellation, deadlines, and tracing spans don't reach the read. Plumb ctx through the read too.
severity: significant
resolution: reconcileOutgoingRelations's snapshot read uses outgoingRelationsCtx(ctx, ...) — the request ctx now reaches the store.
status: addressed
---
