---
id: scheduler-e2e-waits-for-state-write
type: automated-measure
title: 'Test: scheduler e2e waits for the state file, not just the task side effect'
description: 'Regression for BUG-PRFSTS: the scheduler e2e cases must wait for .rela/scheduler-state.json, the run''s LAST write, before cancelling and returning. Waiting only on the note the task creates lets saveState race t.TempDir cleanup and fail with "directory not empty". Fails as an intermittent teardown error rather than an assertion, so treat any such flake here as this regression.'
kind: test
location: internal/appbuild/scheduler_e2e_test.go (TestSchedulerCmd_EndToEnd, schedulerSettled)
status: proposed
---

## What it prevents

`recordSuccess` persists state AFTER the task's visible side effect. A test
that synchronises on that side effect can cancel, return, and begin `TempDir`
cleanup while `saveState` is still writing into `.rela/`.

The failure surfaces as a cleanup error, not an assertion failure — so it reads
like infrastructure noise and invites a retry rather than a fix.

## Why waiting on the note is not enough

The note is what the task DOES; the state file is how the scheduler records
that it is done. Only the second one means the run has finished. Any future
task-completion side effect added after `saveState` moves this line again,
which is the argument for joining in-flight work in `Run` instead (see the
bug's option 2).

## Verifying it still holds

The race is timing-dependent, so repetition is the check — and it must run on
Linux, where it was observed:

```bash
go test -tags sqlite -run TestSchedulerCmd -count=50 ./internal/appbuild/
```

Any "TempDir RemoveAll cleanup ... directory not empty" is this regression.
