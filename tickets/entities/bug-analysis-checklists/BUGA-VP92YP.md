---
id: BUGA-VP92YP
type: bug-analysis-checklist
title: 'Analysis: Scheduler stalls: durable jobs over 30s never complete and block their idempotency key'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Postgres build, PostgreSQL 18.6 local. A test enqueued one `RetryNever` job with
an idempotency key on `jobs.NewPostgresQueue`, with a handler that sleeps 45 s,
then re-enqueued the same key every 10 s for 160 s. Result: 3 executions, row
stayed `status=new retries=0`, and every re-enqueue returned `ErrDuplicateJob`.
neoq logged `error updating job status: FATAL: terminating connection due to
idle-in-transaction timeout (SQLSTATE 25P03)` after each run. Condition: any
handler longer than 30 s on the postgres tier.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

neoq's default 30 s idle-transaction timeout kills the row-lock transaction of
any handler longer than 30 s. The job never completes, reruns forever, and holds
its idempotency key. The scheduler's synchronous in-process wait turns each lost
completion into a 20 m stall of every task.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: see BUG-TKL08E "Fix plan" (part 1: queue timeout; part 2: durable run
lifecycle). Regression test: AM-durable-job-outlives-idle-tx. Related areas: all
durable job kinds share the 30 s kill (mail, AI, for_each children with
`RetryBounded`); TKT-7XLVP7 and TKT-DK0X6O are subsumed by part 2.
