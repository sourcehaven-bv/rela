---
id: RR-4SJK40
type: review-response
title: Sleep-based negative assertion in the worker test
finding: 'internal/appbuild/realtime_triggers_internal_test.go:188. `time.Sleep(3 * time.Second / 2)` followed by require.Empty adds 1.5 s to every run and proves nothing on a slow CI box: a round could still be pending. Fix: observe the worker deterministically, for example a hook or counter on switchLock that records a failed take for the triggers run, and assert on that instead of wall time.'
severity: minor
resolution: The sleep is gone. The two worker test asserts a follower reads nothing with a bounded require.Never on atomic counters and checks the takeover with Eventually.
status: addressed
---
