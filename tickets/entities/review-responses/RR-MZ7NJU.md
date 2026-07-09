---
id: RR-MZ7NJU
type: review-response
title: Multi-page loop ignored Pinia Colada's AbortSignal — superseded fetch runs to the cap
finding: Pinia Colada passes an AbortSignal into the query function context and aborts it when a refetch with the same key supersedes the in-flight call. The board's query neither received nor forwarded it, so a superseded pagination loop kept issuing up to 50 sequential requests producing a result the cache discards. Every drag-drop settleOptimistic invalidation and every SSE entity echo supersedes the running loop — on a busy large board this amplifies into a request storm of racing full-pagination loops.
severity: significant
resolution: listEntities and listAllEntities now accept an optional AbortSignal, forwarded to api.get (axios rejects immediately on an aborted signal, ending the loop between pages as well as in-flight). KanbanView's query fn destructures { signal } from the Colada context and passes it through. Unit test asserts the signal reaches every page request and that aborting after page 1 rejects the loop with no third request.
status: addressed
---
