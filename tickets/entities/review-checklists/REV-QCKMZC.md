---
id: REV-QCKMZC
type: review-checklist
title: 'Review: jobs.Queue seam over neoq: ephemeral memory backend for FS/desktop, durable postgres for networked; migrate scheduler onto it'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test -race ./...` — PASS (exit 0, full suite)
- [x] `just lint` (jobs + appbuild + scheduler) — 0 issues
- [x] `just coverage-check` — PASS; package + total thresholds satisfied, 77.5% total,
`internal/jobs` 84.1% against a 75 floor
- [x] `go-arch-lint check` — OK, no warnings
- [x] `just plimsoll` — PASS
- [x] `just comment-lint` — no findings across 10,325 comments
- [x] `just lint-md` — 0 issues across 253 files
- [x] Build isolation — default build links **0** pgx; postgres build links **0** bleve
- [x] `go build -tags postgres ./...` and `go vet -tags postgres` — clean

## Code Review

- [x] `cranky-code-reviewer` run against the working tree
- [x] Review responses created for every finding
- [x] All critical and significant findings addressed

**12 review responses created and linked.** The review was substantive: it
empirically reproduced four critical defects, all of which were real and all of
which I independently re-verified before fixing.

| ID | Severity | Status |
| --- | --- | --- |
| RR-Y7CJLD | critical | addressed — worker pool died on retry exhaustion |
| RR-QF9B7I | critical | addressed — worker pool died on deadline expiry |
| RR-MCAOI3 | critical | addressed — silent job loss via payload-hash dedup |
| RR-7C9B4Q | critical | addressed — handlers silently capped at 30s |
| RR-HVJOWP | significant | addressed — third upstream data race (Start/Shutdown) |
| RR-1SUTXH | significant | addressed — queue wired but never started |
| RR-JYXHRT | significant | addressed — unsafe retry fallback on corrupt payload |
| RR-JXSNFY | significant | addressed — handler saw wrong Retry policy |
| RR-UDXRQ6 | significant | addressed — Flush error now names lost job kinds |
| RR-Z2I3WH | significant | addressed — conformance suite could not detect a dead pool |
| RR-2AU871 | significant | wont-fix — upstream cron goroutine leak, not fixable externally |
| RR-K4MZMV | minor | deferred — reconsider neoq for the ephemeral tier |

**The core lesson, recorded because it generalises:** the original fix for
neoq's fatal-worker behaviour treated one symptom (`MaxRetries=0`) rather than
the class. neoq returns from its worker goroutine on BOTH
`ErrJobExceededMaxRetries` and `ErrJobExceededDeadline`, so patching one route
left the other open — and the deadline route is the *designed* path, since
`Schedule.NextRun` attaches a short deadline to every scheduled job. The queue
would have gone permanently silent in production under ordinary failure load,
with a green test suite.

The fix inverts ownership rather than chasing routes: neoq receives a retry
budget it can never reach (`backendRetryBudget`) and no `Deadline` at all, and
`dispatch` — the single chokepoint every job passes — owns attempt counting and
deadline enforcement, returning nil for a spent job so neoq never evaluates a
terminal condition of its own.

## Verification

- [x] Each acceptance criterion re-verified after the fixes
- [x] Regression tests added for every fixed defect
- [x] Fixes proven to bite

**Proof the new tests actually catch the bug:** reverting `backendRetryBudget`
from `1<<30` to `1` makes `jobstest/PoolSurvivesExhaustedJobs` fail with its
exact diagnostic — *"the queue stopped processing after jobs exhausted their
retries — worker pool died"* — and restoring it makes the suite green again. A
conformance case that cannot fail is worth nothing; this one was checked.

Four cases added, addressing RR-Z2I3WH's point that the original thirteen all
asserted a property's happy path and none asserted the queue still worked
afterwards:

- `PoolSurvivesExhaustedJobs` — 8 jobs exhaust their budget, then 10 healthy
jobs must all run
- `PoolSurvivesExpiredDeadlines` — every worker blocked, queued deadlines lapse,
then healthy jobs must still run
- `IdenticalPayloadsAreDistinctJobs` — 5 identical payloads → 5 executions
- `HandlerSeesItsRetryPolicy` — the policy survives the round trip

Independent reproduction of the two claims I considered most severe, run against
unwrapped neoq with no rela code involved: retry exhaustion → **0 of 8** healthy
jobs ran; 20 create/close cycles → **exactly 20** leaked goroutines.

## Outstanding

Nothing blocking. Two items deliberately carried forward, both recorded with the
condition that would make them urgent:

- **RR-2AU871** (cron goroutine leak) — bounded and harmless for a single
long-lived process, which is every current caller. It stops being acceptable if
rela ever assembles one `Services` per tenant and evicts them; documented at
`neoqQueue.Close`.
- **RR-K4MZMV** (reconsider neoq for the ephemeral tier) — a fair point. Every
hazard fixed in this ticket was upstream, and the ephemeral tier gains nothing
from the dependency. Not urgent now that the containments are tested and pinned,
but worth a decision entity before the choice ossifies.
