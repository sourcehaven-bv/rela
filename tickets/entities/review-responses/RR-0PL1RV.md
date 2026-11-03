---
id: RR-0PL1RV
type: review-response
title: T4 sweep and T5 worker use two different hosting models
finding: T4 0.5 runs its time-based work on `jobs.Queue`, started in appbuild assembly like softdeletegc and stopped in `Services.Close`. T5 rejects jobs.Queue and starts a bespoke ticker from `cmd/rela-server` `startScheduler` and from desktop main. The two disagree on whether `IdempotencyKey` is enough across nodes, and on whether background realtime writes pause during a Configure save (T5's do, T4's do not). Pick one host and one pause behaviour for all realtime background writers.
severity: significant
resolution: Per D1 one leader worker in appbuild hosts the T4 sweep and the T5 triggers. It starts where softdeletegc starts for long-running processes and pauses during a Configure save.
status: addressed
---
