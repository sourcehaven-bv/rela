---
id: RR-WDVJ0X
type: review-response
title: Guarantee assumes no concurrent non-migration writers
finding: The lock excludes migration/GC runs only; Configure-save runs inside a live server.
severity: minor
resolution: installGraphReads comment and the doc now say the guarantee concerns the migration's own writes and advise running without concurrent edits. A pg REPEATABLE READ snapshot is out of scope.
status: addressed
---
