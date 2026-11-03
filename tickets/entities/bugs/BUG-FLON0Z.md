---
id: BUG-FLON0Z
type: bug
title: Job queue Close returns before running handlers stop; log-capture tests leave log output on a dead buffer
description: A background automation job outlived Services.Close and logged into another test's capture buffer, because neoqQueue.Close does not wait for handlers and the capture tests do not restore the log package output.
priority: medium
effort: s
why1: An automation job's dispatch logged 'job attempt failed' into the bytes.Buffer of TestRunScheduledTemplate_SuppressionLogDoesNotDiscloseHiddenContent after that test had read it.
why2: 'The job belonged to an earlier test (TestBackgroundAction_QueueMode) and was still running after its Services.Close: neoqQueue.Close calls neoq Shutdown, which on the memory backend cancels handler contexts and returns without waiting.'
why3: Its log line reached the finished test's buffer because the queue logs through a logger built from the original slog.Default (the default handler, which writes to the log package output). slog.SetDefault(textLogger) points the log package output at the test's handler, and slog.SetDefault(prev) does not point it back.
why4: Services.Close documents that the queue drains before the store closes, but nothing tested that Close waits for running handlers; the conformance suite only checked enqueue after close. The capture tests each wrote their own swap-and-restore code, so the incomplete restore was copied.
why5: Process-global state (the default logger) is swapped ad hoc in tests without a shared helper that restores all of it, and a shutdown contract (drain before close) relied on library behavior nobody had pinned. The failure needs a slow runner to show.
prevention: Close waits for running handlers, pinned by the jobstest case CloseWaitsForRunningHandlers for every backend. Tests swap the default logger only through internal/testutil/logtest, which restores the slog default and the log package output and flags, pinned by TestCapture_RestoresLogOutput. A forbidigo lint rule bans slog.SetDefault in tests outside logtest.
started: "2026-10-18"
completed: "2026-10-19"
status: done
---

## Description

`TestNewSharedBase_CompilesWorlds` (`internal/appbuild`) failed under `-race` in
CI run 39133822534 (job Test, attempt 1, shuffle seed 1792331958906630552) with
two `DATA RACE` reports on a `bytes.Buffer`:

- Write: `jobs.(*neoqQueue).dispatch` logging `job attempt failed` for an
automation job (`automationJobs.run` → `runOnce` returned `context canceled`),
through `slog.Default`'s default handler and the `log` package output.
- Previous read: `TestRunScheduledTemplate_SuppressionLogDoesNotDiscloseHiddenContent`
(`scheduled_mail_acl_test.go:525`, `logged.String()`), a test that had already
finished.

The failing test is only where the race was noticed. Two defects combine:

1. `neoqQueue.Close` returns while handlers are still running. neoq's memory
backend `Shutdown` only cancels their contexts; it does not wait. (The postgres
backend waits only up to neoq's 1 s cancel grace, as a side effect of closing
its pool.) So an automation job from an earlier test
(`TestBackgroundAction_QueueMode`) keeps running after `Services.Close`, sees
its context canceled, and logs. `Services.Close` documents that it drains the
queue before the store closes; it did not.
2. The log-capture tests in appbuild swap the default logger with
`slog.SetDefault(textLogger)` and restore it with `slog.SetDefault(prev)`.
`SetDefault` with a non-default handler also redirects the `log` package output
to that handler, and restoring the original default handler does not undo that.
From then on, every logger built from the original `slog.Default()` (the
queue's, for one) writes into the finished test's buffer.

## Reproduction

The natural failure needs the late job write to land after the capture test, so
it did not reproduce locally in 20 full-package runs (`-race -shuffle=on`) and
230 targeted runs. Deterministic reproductions:

- `jobstest` case `CloseWaitsForRunningHandlers`: a handler that is still
running when `Close` returns (memory queue: fails 20 of 20 runs on develop;
postgres queue, with the handler running 1.5 s past the cancel: fails 3 of 3).
- `TestCapture_RestoresLogOutput` without the log package restore: a
write through the original default logger after the capture lands in the capture
buffer.
- A probe test (not committed) that replays the CI sequence: a queue-mode
automation job in a busy Lua loop, `Services.Close`, then the `SuppressionLog`
test's capture pattern. Run as separate processes with `-race -test.cpu=1`: 23
of 100 runs report the same race as CI on develop.

## Fix

- `neoqQueue` counts running dispatches; `Close` waits for them after
`Shutdown`, bounded by its context. A dispatch after `Close` is refused.
- New package `internal/testutil/logtest` (`SetDefault`, `Capture`): swaps the
default logger, uses a synchronized writer for captures, and restores the slog
default and the `log` package output and flags. All tests that swapped the
default logger (appbuild and 15 other packages) now use it. A forbidigo rule
bans `slog.SetDefault` in tests outside it.

After the fix: `CloseWaitsForRunningHandlers` 0 of 20 (memory) and 0 of 3
(postgres); the CI-sequence probe 0 of 100 with both fixes, and 0 of 100 with
either fix alone.
