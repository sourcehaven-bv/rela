---
id: RR-NVH9SU
type: review-response
title: Order-dependent subtests share one env
finding: 'TestLive_BusyListWakesOnSessionMove (live_busy_test.go:348-404) and TestLive_BusyListIgnoresMessageOnly (:427-471) run t.Run subtests in sequence on shared state. ''claim'' needs ''move that changes nothing'', and ''edit'' and ''removal'' need seq from an earlier subtest. Running one of them with -run fails or tests the wrong thing. Fix: drop t.Run for these steps (use plain steps with t.Log) or have each subtest build its own preconditions.'
severity: nit
resolution: 'Fixed: TestLive_BusyListWakesOnSessionMove and TestLive_BusyListIgnoresMessageOnly no longer use t.Run for dependent steps. They run plain steps with t.Log, and the failure messages name the case.'
status: addressed
---
