---
id: IMPL-H8FLES
type: implementation-checklist
title: 'Implementation: jobs.Queue seam over neoq: ephemeral memory backend for FS/desktop, durable postgres for networked; migrate scheduler onto it'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented
- [x] All edge cases from planning handled

**What was built:**

- `internal/jobs` — the seam (`Queue`, `Job`, flat `Retry` enum, `Handler`),
the retry mechanism table (`retry.go`), commit deferral (`deferred.go`), and one
neoq adapter (`neoqqueue.go`) shared by both backends so semantics cannot drift
between tiers.
- `memqueue.go` (default build) / `pgqueue.go` (`//go:build postgres`).
- `internal/jobs/jobstest` — conformance harness, 13 cases, following
`storetest`/`statetest`.
- `scheduler.Schedule.NextRun` — the cadence→deadline mapping. Lives in the
scheduler; the queue stays schedule-agnostic.
- `appbuild` wiring: `Services.Jobs()`, per-build `jobQueueFor`, bounded
teardown in `Close`.
- Config: arch-lint component + vendor grant, coverage floor, CLAUDE.md rule,
`docs/background-jobs.md`.

## Three real defects found and fixed during implementation

These are recorded because each was caught by a test that nearly did not exist,
and each would have shipped silently.

1. **`RetryNever` retried.** neoq's `MaxRetries` counts retries AFTER the first
attempt, not total attempts, so the initial mapping ran every policy one time
too many. Caught only after the conformance test was strengthened to outwait the
~16s backoff — the original 500ms wait passed vacuously, because a retry was not
yet due. Pinned now by a direct unit test
(`TestRetry_MaxRetriesIsRetriesNotAttempts`) so it fails fast rather than after
a 25s wait.
2. **`MaxRetries: 0` silently destroyed the queue.** neoq's worker treats
`ErrJobExceededMaxRetries` as fatal and returns, so a zero budget killed a
worker goroutine per job until none remained — observed as "4 of 50 jobs ran",
with **no enqueue error**. RetryNever now submits a budget of 1 to keep the
worker alive, and single-execution is enforced authoritatively in the dispatcher
instead.
3. **Upstream data race in neoq** (`memory_backend.go:116`): `job.ID =
m.jobCount` reads the counter outside the mutex guarding its increment.
Reproduced with pure neoq and zero rela code. rela runs `-race` in CI and
forbids `//go:build !race`, so enqueues are serialized in the adapter
(`enqueueMu`) with a comment pointing at the upstream fix.

A **second** upstream race exists in neoq's panic-recovery path (`handler.Exec`,
handler.go:154/181). It is not containable from our side, so the conformance
suite deliberately has no panicking-handler case, with the reasoning recorded
inline. A panicking handler is a bug in the handler, not a runtime condition the
queue must survive — the gap is narrow and stated.

## Manual Verification

- [x] Feature tested end-to-end
- [x] Each acceptance criterion verified
- [x] Verification evidence documented

**Evidence (all commands run locally):**

| AC | Result |
| --- | --- |
| 1 — flat enum, no call-site knobs | `Enqueue` takes only `Job{Kind,Payload,Retry,Deadline}`; conformance asserts behavior, never numbers |
| 2 — deferred until commit | `jobstest/DeferredUntilCommit` PASS — asserts the handler has NOT fired pre-commit, then fires after `Flush` |
| 3 — enqueue outside a tx runs | `EnqueueRunsHandler` PASS |
| 4 — `RetryNever` runs once | `RetryNeverRunsOnce` PASS (waits 25s, past the backoff — see defect 1) |
| 5 — `RetryBounded` retries then stops | `RetryBoundedRetriesThenStops` PASS |
| 6 — past deadline never runs | `PastDeadlineNeverRuns` + `ZeroDeadlineMeansNoDeadline` PASS |
| 7 — dependency isolation | `go list -deps ./cmd/rela{,-server}` → **0** pgx; `-tags postgres` → **0** bleve; neoq linked in both |
| 8 — scheduler unchanged | `go test -race ./internal/scheduler/` PASS with the existing 939-line suite **unmodified** |

Additional: `TestServices_JobsIsWired` proves the assembled `Services` carries a
queue that actually round-trips a job, not merely a non-nil field.

## Quality

- [x] Code follows project patterns
- [x] No silent failures (errors surfaced, not just logged)

- `just lint` (jobs + scheduler + appbuild): **0 issues**. Fixing `funlen` on
`assemble` meant extracting `buildReadDeps`/`buildRuntimeServices` rather than
raising the limit.
- `go-arch-lint check`: **OK**. `jobs` is a near-leaf; the neoq vendor grant is
scoped to `internal/jobs` alone so no consumer can import neoq directly.
- `just plimsoll`: PASS. `Services` went 25→26 exported methods; the pin was
bumped with a note that the fix is splitting the bundle (TKT-N0IKN9), not hiding
an accessor.
- Coverage: `internal/jobs` **84.1%** against a 75 floor. `jobstest` is
excluded like `storetest`/`statetest`.
- `just lint-md`: 0 issues across 253 files.

**No silent failures:** an unroutable kind fails at `Enqueue` rather than being
stored for a worker that can never run it; a nil logger or backend is rejected
at construction; `Collector.Flush` joins and returns enqueue errors instead of
dropping them.

## Deliberate scope boundaries

- The scheduler's execution engine was **not** swapped. Its retry ladder,
clock-jump guard (BUG-ZKK2UL), and last-*successful*-run semantics are intact
and its suite is untouched — the risk register named an edited scheduler test as
the signal to stop. What landed is `NextRun`, the piece the deadline mapping
needs. Swapping execution is a follow-up with its own regression budget.
- `internal/lua/http.go` and `internal/ai` remain inline, per the ticket: moving
them changes Lua semantics and needs its own ticket.
- `WithPgxPool` is not upstreamed yet, per the recorded decision.
