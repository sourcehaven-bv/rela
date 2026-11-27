---
id: BUG-TIMEFLAKE
type: bug
title: Two flaky tests, one root cause — fixed timeouts sized against uninstrumented, uncontended local timing
description: 'Fixed wall-clock deadlines sized against fast uncontended local runs made tests flaky under -race and CI load. Instance 1 (analyze_cap_test.go) was fixed in #1875. Instance 2 (internal/fswatch/watcher_test.go) now waits on the event with a 30s outer deadline. The docscapture capture deadline is already covered by the cold-start retry from #1618.'
why1: TestWatcher_AutoWatchesNewDirectories waited a fixed 2s for an fsnotify event and the event arrived later under -race and coverage instrumentation on a contended runner.
why2: The 2s value was chosen against fast uninstrumented local runs where the event arrives in milliseconds, so it looked generous.
why3: The wait was written as a latency budget (time.After per select) rather than as a condition with an outer failure bound, and the same pattern was copied into every wait in the file.
why4: Nothing distinguishes a latency budget from a failure bound in review; a short fixed wait passes locally every time and only fails on slow CI runs.
why5: Test deadlines are sized against the author machine instead of the slowest instrumented environment the test runs in (the same class as instance 1 and the docscapture capture deadline).
prevention: Waits in tests block on the condition and use one generous named outer deadline (eventTimeout = 30s) whose doc comment says it is a failure bound, not a latency budget. Absence assertions use a positive sentinel. The docscapture capture deadline is already backed by a bounded retry for cold start (#1618).
started: "2026-11-27"
completed: "2026-11-27"
status: done
severity: medium
---

**Two tests fail intermittently in CI for the same reason: a hard-coded
wall-clock deadline chosen against fast, uninstrumented, uncontended local
runs.** Under `-race` (slower) or CPU contention (slower still), the deadline
slips.

Filed as one bug rather than two because the fix is a class, not two constants.

## Instance 1 — `TestAnalyzeProperties_StopsScanningAtCap`

`internal/dataentry/analyze_cap_test.go:134`. Seeds **5000** entities to prove
the analyzer stops at a cap of **100** — 50× the cap, with the assertion only
checking `rows <= 102`.

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
`EXIT=1` on an integration branch that took real analysis to attribute correctly
— that analysis is only cheap while flakes are rare.

Found while landing FEAT-9CD2MX. Neither test is touched by that work: the
branch modifies no file under `internal/storage`, and no `analyze*` file.

## Another sighting (2026-09-20, TKT-Z8K2FS)

`just ci` on a loaded dev machine, on a branch touching **no file** in
`internal/dataentry`:

```
panic: test timed out after 10m0s
	running tests:
		TestAnalyzeProperties_StopsScanningAtCap (7m3s)
FAIL	github.com/Sourcehaven-BV/rela/internal/dataentry	604.201s
```

The same package alone passes in **66s**, and the test alone in ~67s. So the
7m3s is contention against a shared package budget, not a hang — which is this
bug's thesis, observed again.

Two costs worth recording, since they argue for the fix rather than for raising
the timeout:

- The failure surfaces as a package-level panic over a goroutine dump of bleve
internals, so it reads as a deadlock in search. Establishing that it was
unrelated to the branch under test took longer than the fix would.
- It blocks the whole package, so one slow test's budget overrun is reported as
`FAIL internal/dataentry` with no named culprit in the summary line.

## Merged from BUG-7ZG8Q6 (2026-11-27)

The worlds-manual CI step fails the same way. `internal/docscapture/capture.go`
bounds one navigate, render and capture with `perCaptureTimeout = 30 *
time.Second`. The step runs last in the E2E job, on a runner loaded by the full
Playwright suite and a postgres container. The first `screenshot{}` also pays
the SPA's first-load cost inside the same budget. A re-run with no code change
passes (run 35185989373). Omitting the sweep env vars locally fails at a second
deadline, the version-sweep wait (`manual:936`).

## Resolution (2026-11-27)

**Instance 1** was fixed by b9f2b42bf (#1875): `analyze_cap_test.go` now seeds
500 entities.

**Instance 2** moved to `internal/fswatch/watcher_test.go` (aa2a7a289, #2614)
and still waited a fixed 2s. Every wait in that file (five `time.After(2 *
time.Second)` selects and the `waitWatched` poll) now blocks on the condition
itself through a `waitForEvent` helper, under one named outer deadline,
`eventTimeout = 30s`. The deadline is a failure bound, not a latency budget: a
passing wait returns as soon as the event arrives. Batches without the wanted
event are skipped instead of failing the test. The absence check in
`TestWatcher_IgnoresNonMatchingExtensions` keeps its positive `.md` sentinel.
The modify and delete waits now fail when the expected op never arrives; before,
they only logged it. `go test -race -count=100 -run
TestWatcher_AutoWatchesNewDirectories ./internal/fswatch/` passes.

**Docscapture (merged from BUG-7ZG8Q6)**: no code change. `perCaptureTimeout` is
still 30s, but e8015b901 (#1618, BUG-EB2BGR), landed on 2026-09-19, two days
after the failures recorded above, already separates the first capture's
cold-start cost from the budget. `Capture` retries a transient failure up to
three times, each attempt with its own fresh 30s deadline and a relaunched
browser. A check of the 277 failed `ci.yml` runs from 2026-09-19 to 2026-11-27
found no failure of the "Build the worlds manual" step. An env-tunable ceiling
would add a knob for a failure that has not recurred, so it is left out. The
second deadline, `versionAwaitTimeout` (60s), only expires when the
version-sweep env vars are omitted, and the CI step sets them.
