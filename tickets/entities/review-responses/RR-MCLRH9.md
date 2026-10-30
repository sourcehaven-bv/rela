---
id: RR-MCLRH9
type: review-response
title: Tests silently depend on staying below the first safety tick
finding: 'newBusyLiveEnv (live_busy_test.go:49-50) puts every principal in the last slice, so a safety turn fires at tick inboxSafety-inboxGrid (24 or 16). Tests that add steps past that tick will see extra board replays and frames that have nothing to do with the case under test. Fix: state the tick budget in the newBusyLiveEnv comment, and fail fast in a helper (require that ticks.next() < the first safety tick) in tests that assert zero replays.'
severity: nit
resolution: 'Fixed: the busyLiveEnv comment states the tick budget (24 or 16). requireBeforeSafety fails fast and runs before the zero-replay assertions and inside stepUntil.'
status: addressed
---
