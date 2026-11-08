---
id: RR-LOOV2W6
type: review-response
title: Background work stops while a tenant is evicted
finding: Eviction stops the tenant's job queue, mail outbox, GC and version sweep until the tenant is next used.
severity: minor
reason: Needs a tenant-independent job runner; tracked in TKT-N3X235N next to the scheduler gap TKT-M3SHYE0.
status: deferred
---
