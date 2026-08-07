---
id: RR-VI9XMY
type: review-response
title: 'provision: re-stamped ctx must reach the manager + the read gate must be rebuilt'
finding: For a provisioned principal's triggering write to see its own new entity, the ctx re-stamped with the resolved identity must actually reach the manager's AuthorizeWrite call — a helper that RETURNS a new ctx which the handler discards does nothing (the CRUD handlers pass r.Context() straight to the manager). Separately, the read gate is a distinct memoized acl.Request built at attachACLRequest time on the PRE-provision principal; after provisioning, the response-shaping read still runs on that stale gate and can redact/404 the just-created entity out of its own response. Both must be handled wherever the provision seam lands.
severity: significant
resolution: 'RESOLVED in planning. Seam = candidate (a): a write-only middleware layer wrapping the mutating routes. attachACLRequest resolves the principal as today; when it flags unmatched-verified AND unmatched_principal:provision, the write-middleware (which owns writeMu and the manager — NOT the read middleware, avoiding the deadlock noted in the ticket) does the provision under system:provisioner, then REBUILDS both acl.WithRequest and the read gate on the re-stamped ctx and calls r = r.WithContext(newCtx) before delegating to the downstream handler. Because the handler chain now carries the resolved ctx, both gaps close: (1) the CRUD/sync/action/attachment handlers pass r.Context() straight to manager.AuthorizeWrite, which now sees the resolved principal; (2) the response-shaping read runs on the rebuilt acl.Request, so it cannot redact/404 the just-created entity. Wrapping the mutating routes (not per-handler) is what makes it cover every write path — the same anti-bypass property reject got from the shared choke point. Concurrency (ticket ''Idempotency/races'' item): rely on the sub unique:true constraint — both concurrent first-writes may attempt the create; the loser catches store.ErrConflict, re-resolves, and proceeds. No new lock beyond the existing writeMu; correct on fs/mem/postgres and multi-process. Pinned by a catch-and-re-resolve test.'
status: addressed
---

## Finding

Two wiring gaps the provision seam must close:

1. The re-stamped ctx must reach the manager's `AuthorizeWrite` — a returned ctx
the handler discards is a no-op (CRUD handlers pass `r.Context()` straight
through). Callers must adopt `r = r.WithContext(newCtx)`.
2. The read gate is a separate memoized `acl.Request` built on the pre-provision
principal; the response-shaping read runs on it and can redact/404 the
just-created entity. Rebuild both `acl.WithRequest` and the gate on the new ctx.

## Resolution

Open — solved in TKT-ANUJDS planning with the write-seam. (Re-recorded after
deletion when TKT-0C3II2 became reject-only; reject does no write/re-stamp, so
this was always provision-only.)
