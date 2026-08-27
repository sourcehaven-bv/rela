---
id: REV-HE40LR
type: review-checklist
title: 'Review: storetest: cover Freshness.LastModified and declare the Tx tier in Capabilities'
status: done
---

<!-- @managed: claude-workflow v1 -->

PR: #1419 · branch `test/storetest-freshness-txtier-TKT-8TJ2WN`.

## Automated Checks

- [x] `go test ./internal/store/...` — fsstore 2.9s · memstore 2.5s ·
pgstore 27.9s · storetest 0.8s · storeutil 1.0s, all ok
- [x] `go test -tags postgres ./internal/store/pgstore/` — ok, 30.4s **against
a live PostgreSQL**, so the new `TxRollback` gating is genuinely exercised
- [x] `golangci-lint run ./internal/store/...` — 0 issues
- [x] `golangci-lint run --build-tags postgres ./internal/store/...` — 0 issues
- [x] `just arch-lint` — "OK - No warnings found"

One lint issue was found and fixed during the run: `behaviour` → `behavior` (the
project's misspell linter enforces US spelling).

## Code Review

- [x] Reviewed
- [x] Findings addressed

Self-reviewed. The substantive review effort in this batch went to TKT-415WA7,
where a `cranky-code-reviewer` pass found a critical typed-nil bug — and one
finding from it directly shaped this ticket's verification, so it is worth
recording here rather than only there.

That review mutation-tested one of my assertions and showed it was
**tautologically false**: `x == nil && len(x) != 0` can never be true for a
slice, so a test named "PublishesSpecsBeforeReconciling" passed even with the
publish call deleted outright. The lesson generalizes — a new test going green
proves nothing until you have seen it go red for the right reason.

So every assertion here was checked by deliberate regression. Removing the
relation scan from memstore's `LastModified` fails `CoversRelationWrites` with
the intended message. That is the specific mistake a new backend makes, because
every entity-only test still passes without it.

No `review-response` entities: nothing was deferred or disputed.

## Acceptance Verification

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — five properties covered | **PASS** | empty/zero, entity-write, relation-write, read-stability, timestamp plausibility |
| 2 — three backends pass unchanged | **PASS** | no backend code touched; all green |
| 3 — non-vacuous | **PASS** | deliberate regression fails `CoversRelationWrites` |
| 4 — Tx tier declared, runs from `RunAll` | **PASS** | pgstore's `TestConformance/TxRollback` runs four subtests; separate entry point removed |
| 5 — lint + arch-lint | **PASS** | 0 issues both tags; arch-lint clean |

**Scope stated plainly:** no backend is fixed here. All three were already
correct; these tests pin that so a *fourth* cannot be silently wrong. The value
is entirely forward-looking, which is the right time to add them — before the
SQLite backend exists, not after it has shipped a subtle freshness bug.

## Documentation

- [x] ~~User-facing docs~~ (N/A: test-only, no behavior change)
- [x] Rationale documented where the next backend author reads it — why the
assertions are coarse (clock-driven backends cannot promise more), why
`waitForClock` exists (filesystem mtime granularity), and why `TxRollback` is a
declared claim rather than an inferred one
