---
id: AM-fixture-timeouts-are-not-wall-clock
type: automated-measure
title: Timing-sensitive tests assert on observable state, not a fixed wall-clock deadline
description: 'The two known-flaky tests must stop encoding a hard-coded deadline sized against fast, uninstrumented, uncontended local runs. Pins BUG-TIMEFLAKE: analyze_cap_test.go seeds 5000 entities to prove a cap of 100 (a measured 88x reduction is available) and watcher_test.go hard-codes a 2s fsnotify wait, so both fail on a loaded or instrumented runner while the code is correct.'
kind: test
location: internal/dataentry/analyze_cap_test.go, internal/store/fsstore/watcher_test.go, internal/lock/memlock_test.go (all to be rewritten with the fix)
status: proposed
---

Pins BUG-TIMEFLAKE and BUG-JEB6UD.

A test that fails under `-race`, under coverage instrumentation, or on a loaded
CI runner — while the code under test is correct — is measuring the machine, not
the behaviour. Every site must either wait on the condition it actually cares
about or shrink the fixture until the deadline is not load-bearing.

BUG-JEB6UD is the same class reached by a different route: rather than a
deadline that is too short, it uses a signal that does not mean what the
assertion needs. `close(acquired)` proves the release was CALLED, and the test
then asserts on eviction, which may not have happened yet. The remedy is the
same — wait on the condition being asserted, not on a proxy for it.
