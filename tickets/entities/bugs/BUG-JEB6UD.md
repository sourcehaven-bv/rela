---
id: BUG-JEB6UD
type: bug
title: 'TestMemoryLocker_AbandonedAcquireReleases is flaky: asserts map eviction that happens asynchronously'
priority: medium
status: backlog
---

`go test ./internal/lock/ -run TestMemoryLocker_AbandonedAcquireReleases
-count=8` fails 6 times out of 8 on an unmodified `develop` (verified at
5f04ec38 on a detached checkout, so it is not branch-specific).

```text
memlock_test.go:79: 1 map entries remain after an abandoned acquire (want 0)
```

## Cause

The test races the code it measures rather than observing a stable state.
`internal/lock/memlock_test.go:63-79`:

```go
go func() {
    rel2, err := l.Acquire(context.Background(), "contended")
    if err == nil {
        rel2()          // eviction starts here...
    }
    close(acquired)     // ...but the test unblocks here
}()

select {
case <-acquired:
...
if n := lock.MemoryLockerEntries(l); n != 0 {   // reads too early
```

`close(acquired)` only proves `rel2()` was CALLED. If the entry is evicted on
another goroutine (or after the release returns), the assertion reads the map
mid-eviction and sees the entry still present.

Not a defect in `MemoryLocker` itself: the wedge check above it (the 5-second
`key wedged` guard) passes every time, so the lock IS handed over and released.
Only the bookkeeping assertion is unstable.

## Fix sketch

Either make the test wait for the eviction it is asserting (poll
`MemoryLockerEntries` with a deadline, the way the wedge check already uses a
deadline), or have `MemoryLocker` expose a synchronous point after which
eviction is guaranteed. Prefer the former — the production contract is "the key
does not wedge", which the existing guard already covers, and adding a
synchronisation point for a test would change the code to suit the test.

## Impact

CI flake. The race detector is on in CI, which widens the window, so this likely
fails more often there than locally.

Found incidentally while running the full `./internal/...` suite during
TKT-VAKI0Q; unrelated to that work.
