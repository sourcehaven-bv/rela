---
id: RR-K2WKEJ
type: review-response
title: Reusing the long-lived request-scoped acl.Request freezes group membership for the connection lifetime (stale-allow) and violates its single-goroutine contract
finding: 'An SSE connection is ONE http.Request = ONE acl.Request living for hours. (a) acl.Request.Globals caches the member-of closure on first call for the Request lifetime (request.go:57-63) — if the attacker is removed from a group mid-connection, every subsequent PermitsRead/ReadQuery uses the stale membership → stale-allow → keeps streaming entities they can no longer read. (b) acl.Request is documented single-goroutine-only (request.go:28-34); any second goroutine touching it races on the non-atomic globals memoization (CI runs -race). Fix: the SSE handler must NOT reuse readGateFromContext''s frozen gate — open a fresh acl.Request per event (or periodically re-resolve) so the member-of walk re-runs against current state.'
severity: critical
resolution: 'Final design: the type verdict (ReadQuery(type)) is resolved per-connection and cached, refreshed on the rare membership-changing store events (member-of/role-relation writes that pumpStoreEvents already observes). Far cheaper than the per-entity case the finding worried about — no member-of walk on the hot path, just a cached type-verdict invalidated on the rare events that actually change it. AC6 pins both the caching and the membership-refresh (principal removed from a conferring group stops receiving a type they lose). Single-goroutine concern moot: the gate runs only in the per-connection handleSSE goroutine.'
status: addressed
---
