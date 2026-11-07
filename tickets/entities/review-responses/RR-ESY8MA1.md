---
id: RR-ESY8MA1
type: review-response
title: Per-case goroutines leak and soft-delete GC runs on the wall clock
finding: Assemble starts queue workers, soft-delete GC ticker, datamigration GC, mail outbox, realtime workers.
severity: minor
resolution: Services.Close per case is required; goroutine-leak test across 100 cases. Documented that t.advance does not purge soft-deleted entities.
status: addressed
---
