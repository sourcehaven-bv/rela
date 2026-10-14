---
id: BUG-8M3HEQ
type: bug
title: TestWatchRun_BudgetRotates flakes under load
description: 'internal/mcp TestWatchRun_BudgetRotates relies on a 30 ms budget against a 40 ms sleep; under CI load one pass read two targets (order [a b c a]) and dequeued #1882 from the merge queue.'
priority: medium
status: backlog
---

## Description

`internal/mcp/tools_watch_test.go` `TestWatchRun_BudgetRotates` failed in the
merge-queue run for #1882 (job 115791072256): `replay order = [a b c a], want
each target once in turn`. The PR does not touch `internal/mcp`; the test passes
30 times locally.

The test measures a 30 ms wall-clock budget against a 40 ms `time.Sleep`. A pass
that reads two targets means the budget check saw less elapsed time than the
sleep, or the pass budget starts after the first read. Either way the assertion
depends on scheduling.

## Root cause

`watchRun.pass` (internal/mcp/tools_watch.go) checks only `rctx.Err()` before the next read. That error comes from a timer, which can fire late on a busy machine, so the pass starts a second read past its budget. This is a code fault, not only a test fault: 2 failures in 600 local runs.

## Fix

Also check the clock before each read: `if rctx.Err() != nil || !time.Now().Before(end) {`. With it, 1,200 runs passed.
