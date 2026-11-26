---
id: BUG-F1FJLKF
type: bug
title: TestServer_AwarenessExpiry_GoroutineStoppedOnEvict flakes under -shuffle
description: The test compared process-wide runtime.NumGoroutine before and after evicting a room, so goroutines from other tests that were still winding down made it fail in the merge queue.
priority: medium
effort: xs
why1: The test counted all goroutines in the process and saw 10 after eviction against a baseline of 8.
why2: 'Goroutines from earlier tests were still exiting when the baseline was taken, and other goroutines were started or ended during the poll. The expiry goroutine itself had stopped: CloseRoom calls awareness.Destroy.'
why3: The expiry goroutine runs inside the external ygo library, which exposes no done channel, so the test fell back to runtime.NumGoroutine.
why4: -shuffle=on changes which tests run before this one, so the noise appeared only in some orders.
why5: Leak tests in this package measure a process-wide counter instead of the goroutine under test.
prevention: The two awareness leak tests now count only StartAutoExpiry goroutines that carry a pprof label set by the test. Other tests cannot create goroutines with that label.
started: "2026-11-26"
completed: "2026-11-26"
status: done
---

## Description

On 2026-11-25 the merge queue (run 45711538573, job 139546926455, `go test -v
-shuffle=on`) failed with `expiry goroutine leaked: before=8 after=10` and
dequeued sourcehaven-bv/rela#2605.

The expiry goroutine does not leak: `CloseRoom` calls `awareness.Destroy`, which
closes the goroutine done channel. The test counted every goroutine in the
process, so goroutines from earlier tests that were still exiting raised the
count. `TestServer_Shutdown_DestroysAwarenessOfResidentRooms` used the same
count.

## Fix

Both tests create the room under a pprof label unique to the test run. The
expiry goroutine inherits that label. The tests count only goroutines in
`StartAutoExpiry` that carry the label: they require one before eviction or
Shutdown and none after.

## Reproduction

`go test -race -shuffle=on -count=40 ./third_party/ygows` did not hit the
failure locally. The mechanism needs only another goroutine that exits or starts
between the baseline and the poll; the test allowed a slack of one. The mutation
check confirms the new test: removing `rm.awareness.Destroy()` from `CloseRoom`
(or `r.awareness.Destroy()` from `Shutdown`) makes it fail with `expiry
goroutine leaked: 1 still running`.
