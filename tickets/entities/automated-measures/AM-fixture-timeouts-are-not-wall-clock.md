---
id: AM-fixture-timeouts-are-not-wall-clock
type: automated-measure
title: Timing-sensitive tests assert on observable state, not a fixed wall-clock deadline
kind: test
location: internal/dataentry/analyze_cap_test.go, internal/store/fsstore/watcher_test.go (to be rewritten with the fix)
status: proposed
description: "The two known-flaky tests must stop encoding a hard-coded deadline sized against fast, uninstrumented, uncontended local runs. Pins BUG-TIMEFLAKE: analyze_cap_test.go seeds 5000 entities to prove a cap of 100 (a measured 88x reduction is available) and watcher_test.go hard-codes a 2s fsnotify wait, so both fail on a loaded or instrumented runner while the code is correct."
---

Pins BUG-TIMEFLAKE.

A test that fails under `-race`, under coverage instrumentation, or on a
loaded CI runner — while the code under test is correct — is measuring the
machine, not the behaviour. Both sites must either wait on the condition they
actually care about or shrink the fixture until the deadline is not load-bearing.
