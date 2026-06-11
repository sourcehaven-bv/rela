---
id: RR-1QTG37
type: review-response
title: 'Minor: stale feedEvent doc (6 vs 7 fields), catch-up EntityType empty, scan-error re-emit storm, package doc, overlap-trigger doc'
finding: 'Collected minors. cranky #6/architect-nit: feedEvent doc (feed.go:68-75) says 6 wire fields but code uses 7 (entityType inserted). architect-nit: catch-up events carry empty EntityType while NOTIFY events carry it — two paths produce structurally different events for the same change; add `type` to the catch-up entities SELECT. cranky #8: catchUp on mid-iteration scan error returns OLD watermark after partial emit — idempotent, but a PERSISTENT scan failure means the watermark never advances and every catch-up re-emits the whole tail forever (Debug-logged, invisible). architect-minor: pgstore.go package doc now contradicts ownership (store DOES own the listener conn). cranky #11/architect: document the real xid8-upgrade trigger (a single write tx holding a seq longer than 100 later writes take to commit, e.g. bulk import in one tx) so the overlap assumption is falsifiable. cranky #7: entity Type isn''t control-char validated before going on the wire.'
severity: minor
resolution: 'Fixed the minors: feedEvent doc corrected to 7 fields incl. entityType; catch-up entities SELECT now carries `type` so catch-up events have EntityType (parity with notification events); pgstore.go package doc rewritten from single-writer to the change-feed/listener-ownership reality; catchUpOverlap doc now states the precise xid8-upgrade trigger (a single write tx holding a seq longer than 100 later writes take to commit, e.g. bulk import in one tx); codec doc notes entity Type is assumed control-char-free (degrades to catch-up otherwise); StopWatching gets a lifecycle-only single-goroutine comment. (catchUp persistent-scan-failure Warn escalation left as-is — idempotent, not corrupting, low value.)'
status: addressed
---

## Resolution

Cheap cleanups in the fix pass:
- Fix feedEvent doc to 7 fields incl. entityType.
- Add `type` to catch-up entities SELECT so catch-up events carry EntityType
(parity with notification events).
- pgstore.go package doc: update the ownership/single-writer wording to match
the listener now living in the store.
- Document the xid8-upgrade trigger precisely (long-running write tx) at the
catchUpOverlap const.
- Note in the codec comment that entity Type is assumed control-char-free
(degrades to catch-up otherwise) — or validate it; low risk.
- catchUp persistent-scan-failure: log at Warn on repeated failure (lower
priority; idempotent so not corrupting).
- StopWatching field access is lifecycle-only single-goroutine (cranky #9) —
add a clarifying comment.
