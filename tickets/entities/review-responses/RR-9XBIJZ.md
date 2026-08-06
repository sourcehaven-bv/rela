---
id: RR-9XBIJZ
type: review-response
title: Re-stamped ctx never reaches the manager call; response-shaping read stays on the stale anonymous gate
finding: 'Two halves of one wiring gap. (a) The plan re-stamps ctx in the helper and RETURNS it, but every CRUD handler passes r.Context() straight to the manager (write_handler.go:138 etc). A returned context that the handler discards does nothing — AuthorizeWrite still sees the unmatched principal. Each handler must r = r.WithContext(newCtx). (b) The read gate is bound to the acl.Request built at attachACLRequest time on the PRE-provision principal (readgate.go, router.go:283-296). After a successful create the handler shapes the 201 response via the reader/serializer using that STALE gate — so the write can succeed (fresh write authz sees groups) while the response-shaping read runs as the anonymous principal, redacting/404ing the just-created entity out of its own response. The plan''s Q2 reasoning only covers AuthorizeWrite, never that the read gate in ctx is a separate memoized Request provisioning doesn''t rebuild. Fix: after provisioning, rebuild BOTH acl.WithRequest and withReadGate on the new ctx.'
severity: significant
resolution: DEFERRED to the provision ticket (parked). Both halves — the re-stamped ctx reaching the manager, and rebuilding the read gate — are provision-only concerns (they arise from provisioning re-stamping the principal mid-request). reject performs NO write and NO re-stamp, so neither applies to this ticket. Captured in the parked design doc's 'unresolved seam' section.
status: deferred
---

See title. Fix: the provision helper must (a) have callers adopt the returned
ctx (`r = r.WithContext(newCtx)`), and (b) rebuild both the `acl.WithRequest`
Request and the `withReadGate` gate on the new ctx, not just re-stamp the
Principal — otherwise the response-shaping read runs as the anonymous principal.
Interacts with RR-BZQ049: a choke-point seam fixes both at once.
