---
id: RR-9XGRXW
type: review-response
title: Mixed old/new binaries diverge permanently; retaining the legacy key buys only point-in-time rollback
finding: 'The plan imports scheduler-state.json on first read and RETAINS the legacy key ''for rollback''. With an old binary (one document) and a new one (per-task rows) live against the same database, the two storage locations diverge immediately and permanently: neither sees the other''s runs, so every task executes on both nodes at both cadences — duplicate side effects for the whole overlap window, and scheduled Lua scripts are not all idempotent. Rollback after a day reads a day-stale snapshot, so every daily task fires at once and mid-ladder tasks reset. Rolling forward again ignores everything the old binary did. ''Retained for rollback'' therefore delivers point-in-time rollback, which is not what an operator means by rollback.'
severity: critical
status: open
---
