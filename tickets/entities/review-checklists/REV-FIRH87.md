---
id: REV-FIRH87
type: review-checklist
title: 'Review: Migration lock as a pluggable mini-service (postgres advisory lock, fs lock file)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` — all green (default build)
- [x] DB-gated pg lock tests (`-race -tags postgres`, local rela_test): `TestMigrationLock_ExclusiveWithinSchema`, `TestMigrationLock_SchemasAreIndependent` — PASS
- [x] `just lint` — 0 issues; `just arch-lint` clean; `just plimsoll` clean (pgstore pin 38/48 → 39/49, justified at the directive)
- [x] `just docs-check` clean after commit (regeneration matches guides)

## Code Review

- [x] cranky-code-reviewer over the branch diff (agent stalled once mid-run; resumed and completed)
- [x] All critical/significant findings addressed

**Findings & dispositions** (RRs on TKT-CPCBR7):

| RR | Severity | Finding | Disposition |
|---|---|---|---|
| RR-SDPFDD | critical | gate wrote the drift ledger outside the lock taken for the marker | FIXED: one withLock section covers marker+ledger; contended gate skips both; test |
| RR-0EPN1X | significant | fs stale-break TOCTOU (double-acquire) + unconditional release remove | FIXED: break-mutex file with re-verify-under-mutex; payload-guarded release; 20-round race test with -race |
| RR-OP2U10 | significant | pool_max_conns=1 self-deadlock | FIXED: preflight refuses < 2 conns with remedy in the message |
| RR-5SD2TZ | significant | pid-reuse wedge, no documented remedy | ACCEPTED (fail-safe direction) + documented in godoc and guide; pid+starttime named as upgrade |
| RR-NNS3K0 | significant | gc --scan aborted on routine sweep contention | FIXED: CLI reports skip and continues to tick preview |
| RR-FHOPAN | significant | six test gaps claimed | 2 real → closed with new tests; 4 rebutted (tests pre-existed in lock_test.go); helper-skip accepted |
| RR-4FDSHO | minor | 7-item batch (LockFor doc, shutdown noise, per-call lock values, generation-guarded release, error wrapping, mkdir hoist, guide accuracy) | all fixed; LockFor's memstore-gets-fs-lock behavior kept and EXPLAINED (the guarded state lives in .rela/ FSKV) |

Reviewer-verified clean areas: constructor nil-validation, no leaked pg conns on
error paths, non-shared sweep key rationale, no deadlock in the
apply→evaluateGate ordering, plimsoll justification, attribution/audit
conventions, appbuild wiring and kill switch.

## Acceptance Verification (PLAN-34E1YZ AC1–AC8)

1. Same-schema exclusion + release + double-release — PASS (pg DB-gated, -race)
2. Schema independence (BUG-CA3VY0 class) — PASS (pg DB-gated)
3. GC/migration mutual exclusion via shared lock — PASS (TestGC_ApplySkipsWhenLockHeld + TestRunner_ApplyFailsFastWhenLockHeld)
4. fs exclusivity, stale/unparseable/zero-pid break with retry, release removes file — PASS (lock_test.go incl. concurrent-break single-winner)
5. Dry-runs/read paths lock-free — PASS (spy-lock tests for Runner and GC tick)
6. Gate contention: verdict published, marker AND ledger unwritten, no error, persists after release — PASS (two gate tests)
7. LockFor selection three-way — PASS (table test)
8. Existing suites updated and green; all linters clean — PASS

Manual e2e (evidence in IMPL-68H55Q): live-pid held lock → apply exits 1 with
clear message, zero writes; dead holder → stale-break warning, apply succeeds,
lock file cleaned up.

## Verification

- [x] PR: https://github.com/sourcehaven-bv/rela/pull/1397
- [x] All review-responses resolved (7 code-review RRs; zero open critical/significant; 3 planning-phase design findings resolved inside PLAN-34E1YZ)
