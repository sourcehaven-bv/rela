---
id: RR-ZO6JI4E
type: review-response
title: Memory Shutdown cancels consumers before stopping cron
finding: A cron tick between cancel and cron.Stop can block on a full queue channel. Stop cron first; postgres has the same order.
severity: minor
reason: rela registers no neoq cron jobs, so no tick can fire. Moved to TKT-TD68KKT.
status: deferred
---
