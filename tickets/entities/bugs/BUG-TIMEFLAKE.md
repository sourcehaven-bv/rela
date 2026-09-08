---
id: BUG-TIMEFLAKE
type: bug
title: Two flaky tests, one root cause — fixed timeouts sized against uninstrumented, uncontended local timing
severity: medium
status: backlog
description: 'Two tests fail intermittently in CI for the same reason: a hard-coded wall-clock deadline sized against fast, uninstrumented, uncontended local runs. analyze_cap_test.go seeds 5000 entities to prove a cap of 100 (a measured 88x reduction is available); watcher_test.go hard-codes a 2s fsnotify wait.'
---

**Two tests fail intermittently in CI for the same reason: a hard-coded wall-clock
deadline chosen against fast, uninstrumented, uncontended local runs.** Under
`-race` (slower) or CPU contention (slower still), the deadline slips.

Filed as one bug rather than two because the fix is a class, not two constants.

## Instance 1 — `TestAnalyzeProperties_StopsScanningAtCap`

`internal/dataentry/analyze_cap_test.go:134`. Seeds **5000** entities to prove the
analyzer stops at a cap of **100** — 50× the cap, with the assertion only checking
`rows <= 102`.

Measured:

| Condition | Duration |
|---|---|
| isolated, no `-race` | 18.41s |
| isolated, **`-race`** (what CI runs) | **209.53s** |
| CI, `-race` + contention | **569s (9m29s)** |

`-race` alone is an **11.4×** multiplier; contention adds ~2.7×. Against Go's
10-minute default that leaves ~30 seconds of headroom in a package whose whole
suite takes 259–297s uncontended.

**A fix was measured but deliberately not committed:** `seeded = 300` runs in
**2.39s** under `-race` — an **88× reduction** — with identical detection power,
verified by removing the early break and confirming it still fails with the same
diagnostic.

## Instance 2 — `TestWatcher_AutoWatchesNewDirectories`

`internal/storage/watcher_test.go:369`. A literal `time.After(2 * time.Second)`
waiting on an fsnotify event.

**Proved flaky by a pass and a fail inside a single run on identical bytes** —
`just ci` runs the suite twice, and:

```
line 101  ok    internal/storage  3.057s   (test pass)
line 206  FAIL  internal/storage  3.483s   (test-coverage pass, SAME run)
```

The coverage pass is slower because of instrumentation, which is exactly when a
tight deadline slips. 5/5 in isolation.

## Why one ticket

Both are *a fixed duration standing in for a condition*. The fix is the same
shape in both places: **wait on the condition, not on a clock** — or, where a
deadline is genuinely needed, scale it against the instrumentation and load the
test actually runs under rather than a number that felt generous on a laptop.

Bumping the two constants would make the symptom rarer without making either
test correct, and the next slow runner brings both back.

## Why it matters beyond noise

A flaky red trains people to re-run rather than read. This arc had a genuine
`EXIT=1` on an integration branch that took real analysis to attribute
correctly — that analysis is only cheap while flakes are rare.

Found while landing FEAT-9CD2MX. Neither test is touched by that work: the
branch modifies no file under `internal/storage`, and no `analyze*` file.
