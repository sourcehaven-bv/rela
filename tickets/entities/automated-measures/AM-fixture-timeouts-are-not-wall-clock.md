---
id: AM-fixture-timeouts-are-not-wall-clock
type: automated-measure
title: Timing-sensitive tests assert on observable state, not a fixed wall-clock deadline
description: 'The two known-flaky tests must stop encoding a hard-coded deadline sized against fast, uninstrumented, uncontended local runs. Pins BUG-TIMEFLAKE: analyze_cap_test.go seeds 5000 entities to prove a cap of 100 (a measured 88x reduction is available) and watcher_test.go hard-codes a 2s fsnotify wait, so both fail on a loaded or instrumented runner while the code is correct.'
kind: test
location: internal/dataentry/analyze_cap_test.go, internal/store/fsstore/watcher_test.go, internal/lock/memlock_test.go, internal/docscapture/capture.go (all to be rewritten with the fix)
status: proposed
---

Pins BUG-TIMEFLAKE, BUG-JEB6UD and BUG-7ZG8Q6.

A test that fails under `-race`, under coverage instrumentation, or on a loaded
CI runner — while the code under test is correct — is measuring the machine, not
the behaviour. Every site must either wait on the condition it actually cares
about or shrink the fixture until the deadline is not load-bearing.

BUG-JEB6UD is the same class reached by a different route: rather than a
deadline that is too short, it uses a signal that does not mean what the
assertion needs. `close(acquired)` proves the release was CALLED, and the test
then asserts on eviction, which may not have happened yet. The remedy is the
same — wait on the condition being asserted, not on a proxy for it.

BUG-7ZG8Q6 is the same class reached in production CI rather than in a unit
test: `docscapture.perCaptureTimeout` is a fixed 30s ceiling on a poll that
already waits for an observable signal (`page-state-*` leaving `pending`). The
budget is sized against an idle machine, and the step that spends it runs last
in the E2E job, after the whole Playwright suite has loaded the runner. Here the
remedy is narrower than a rewrite: the gate is already condition-based, so only
the ceiling needs to stop being a constant.
