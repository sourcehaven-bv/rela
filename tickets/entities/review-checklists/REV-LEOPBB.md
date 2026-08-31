---
id: REV-LEOPBB
type: review-checklist
title: 'Review: Keyed lock seam (internal/lock): named mutual exclusion with per-tier backends'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test -race -count=1 ./internal/lock/...
./internal/store/pgstore/` green, with `RELA_TEST_DATABASE_URL` set against a
live PostgreSQL 15 (94s). Also `just arch-lint` and `just plimsoll` clean.
- [x] Lint clean (`just lint`, exit 0)
- [x] Comment lint gate clean (`just comment-lint` — 11485 comments, no
unresolvable doc links)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A for the full-repo
gate: it exceeds 7 minutes here and was not run to completion. Measured directly
instead — `internal/lock` is at **100% statement coverage**;
`internal/store/pgstore` is excluded from coverage by `.testcoverage.yml` in
favour of the dedicated postgres CI job. No package floor can be breached by
this diff. The full gate should still run in CI.)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Critical review performed (dedicated review agent over `git diff
develop...HEAD`, targeting the concurrency-sensitive paths)
- [x] All critical review-responses addressed — RR-U99GDV fixed in 28ac61f0
- [x] ~~All significant review-responses addressed~~ (N/A: none raised at
significant; findings were 1 critical, 1 minor, 1 nit, 1 doc-clarification)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-U99GDV (critical, fixed), RR-1TQPWE (ValidateKey is
strictly weaker than `state.ValidateKey` — doc corrected), RR-2LIP5S (minor —
the `MaxConns >= 2` guard covers one holder, not N; documented, not enforceable
per-call), RR-GLYY0N (nit — audit record for the memlock refcount /
abandoned-waiter paths, the `Hijack().Close()` sites, and mutation-testing that
`locktest` really catches a degraded backend).

## Acceptance Verification

- [x] ~~Each acceptance criterion tested (reference planning checklist)~~ (N/A:
no planning checklist — ticket was created and implemented in one session at the
user's request.)
- [x] Test evidence documented in implementation checklist (IMPL-XGSYWM)

**Acceptance Status:**

- Keyed mutual exclusion, both tiers — **PASS**. One `locktest.RunAll` suite
  drives the memory and postgres backends; both green under `-race`.
- Distinct keys do not contend (the property the seam exists for) — **PASS**,
  in-process and across two stores on postgres. Mutation-tested: a locker that
  ignores the key IS caught by the suite.
- Cross-process exclusion — **PASS** against a live server
  (`TestKeyedLock_ExclusiveAcrossStores`).
- Tenant isolation — **PASS** (`TestKeyedLock_ScopedPerSchema`): one key in two
  schemas is two locks.
- Cancelled acquire neither wedges the key nor leaks a connection — **PASS**
  (`TestKeyedLock_CancelledAcquireDoesNotWedgeOrLeak`, added during review).
- Bounded waiting via ctx — **PASS** (`ContextCancelWhileWaiting`,
  `ExpiredContextFails`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing
surface. `internal/lock` is an internal seam with no consumer, no config key, no
CLI flag and no API. User-facing docs land with the consumer, TKT-1EM4KL.)
- [x] ~~User-facing documentation updated~~ (N/A: same reason. The contract is
documented as godoc on the interface and the conformance suite.)
- [x] ~~Docs-checklist marked as done~~ (N/A: none created.)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: deliberately not
opened. Work sits on `feat/keyed-lock-TKT-1K47YD` for the user to review; no
push, no PR, no merge was requested.)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
