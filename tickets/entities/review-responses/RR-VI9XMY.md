---
id: RR-VI9XMY
type: review-response
title: 'provision: re-stamped ctx must reach the manager + the read gate must be rebuilt'
finding: For a provisioned principal's triggering write to see its own new entity, the ctx re-stamped with the resolved identity must actually reach the manager's AuthorizeWrite call — a helper that RETURNS a new ctx which the handler discards does nothing (the CRUD handlers pass r.Context() straight to the manager). Separately, the read gate is a distinct memoized acl.Request built at attachACLRequest time on the PRE-provision principal; after provisioning, the response-shaping read still runs on that stale gate and can redact/404 the just-created entity out of its own response. Both must be handled wherever the provision seam lands.
severity: significant
resolution: 'RESOLVED in planning, seam CORRECTED after mapping the actual writeMu/handler topology. The originally-drafted ''write-only middleware wrapping mutating routes'' is NOT viable: writeMu is a plain non-reentrant sync.Mutex taken at the top of ALL 14 mutation handlers (write_handler.go, sync_handlers.go, actions.go, handlers_attachment.go, webhook.go) — a middleware that also takes writeMu would self-deadlock; and routes are not method-split at registration (entity GET and its mutating verbs share the /api/v1/ catch-all, split only by runtime r.Method), so ''wrap only mutating routes'' can''t be cleanly registered anyway. ACTUAL SEAM: a shared provision helper (maybeProvision) invoked at the top of each write handler, INSIDE the writeMu it already holds. Because every write path already holds writeMu at that point, provision is serialized in-process for free (no second lock, no deadlock). The helper: (1) checks acl.UnmatchedVerifiedFrom(ctx) && policy is provision; (2) creates the bare stub under system:provisioner via the manager (tolerating store.ErrConflict + re-resolve for the concurrent case); (3) returns a ctx re-stamped to the resolved entity ID with acl.WithRequest + the read gate REBUILT on it. Each handler adopts the returned ctx as its base (r = r.WithContext(provCtx)); sync/webhook layer their syncContext/webhook-stamp on top of the PROVISIONED ctx. This closes both gaps: manager.CreateEntity/UpdateEntity/etc. see the resolved principal (gap 1), and the response-shaping h.reader.getEntity/h.serializer.forWire read-back runs on the rebuilt gate so it cannot redact/404 the just-created entity (gap 2). Anti-bypass property preserved: every write path calls the one helper, mirroring what reject got from the single AuthorizeWrite choke point. Concurrency: rely on the sub unique:true constraint — loser catches store.ErrConflict, re-resolves, proceeds; no lock beyond the writeMu already held. Pinned by a catch-and-re-resolve test.'
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
