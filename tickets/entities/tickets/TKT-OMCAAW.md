---
id: TKT-OMCAAW
type: ticket
title: Investigate flaky TestMemoryQueue_Conformance/IdempotencyKeyFreedAfterCompletion
kind: test
priority: low
effort: s
tags:
    - needs-investigation
status: backlog
---

## Observed

During `just ci` (`go test -race -cover -shuffle=on`):

```
--- FAIL: TestMemoryQueue_Conformance (68.01s)
    --- FAIL: TestMemoryQueue_Conformance/IdempotencyKeyFreedAfterCompletion (0.00s)
        jobstest.go:100:
            Error Trace: internal/jobs/jobstest/jobstest.go:686
                         internal/jobs/jobstest/jobstest.go:100
            Error: Received unexpected error:
                   jobs: an identical job is already pending
FAIL	github.com/Sourcehaven-BV/rela/internal/jobs	69.174s
```

The subtest reports 0.00s while the parent takes 68s, so the enqueue appears to
race something in the surrounding suite rather than time out on its own.

## Not reproducible in isolation

- `go test ./internal/jobs/ -run '.../IdempotencyKeyFreedAfterCompletion' -count=5` — passes
- `go test ./internal/jobs/ -count=1` on `develop` — passes (68s)
- `go test -race -shuffle=on ./internal/jobs/ -count=1` on the branch — passes (69s)

It needs the full-suite ordering, and `-shuffle=on` varies that per run.

## Filed as a ticket, not a bug

The `bug` type requires `fixes` and `adds-measure` relations, which presuppose a
diagnosed root cause and a chosen prevention. Neither exists yet — this is an
observation awaiting triage. Convert it to a `bug` once the cause is known and
the 5-whys can be filled in honestly.

## Not caused by the branch that found it

Found while running `just ci` for TKT-RX7I97, which touches `internal/secrets`,
`internal/mail`, `internal/cli` and docs — nothing in `internal/jobs`, and no
shared state with it. Confirmed by running the jobs package on `develop`.

## Suspected area

The name says the key should be released once the job completes, and the error
is the still-pending guard firing — so a completion/release step may not be
synchronized with the next enqueue, letting a fast re-enqueue observe the prior
job as still pending.

Per CLAUDE.md, `IdempotencyKey` semantics are load-bearing for recurring tasks
("one of these pending at a time is enough"), so a genuine
release-after-completion race is worth understanding rather than papering over
with a longer wait in the test.

## Next step

`go test -race -shuffle=on -count=20 ./internal/jobs/`, and when it reproduces
note the shuffle seed the failing run prints so it can be replayed
deterministically.
