---
id: REV-783O4V
type: review-checklist
title: 'Review: Data migration system: shape hash, compatibility classifier, declarative migrations, GC sweep'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` (go test ./... all green; race detector on for pgstore suite)
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — clean (new datamigration component + rules)
- [x] `just plimsoll` — clean (Metamodel pin 31→32 documented at directive)
- [x] `just coverage-check` — PASS (datamigration 61.5% vs 50 floor; total 77.2%)
- [x] DB-gated pgstore suite (`just test-postgres` against local rela_test) — PASS with -race, incl. new UpdateChangesType conformance case

## Code Review

- [x] cranky-code-reviewer run over the full branch diff
- [x] All critical/significant findings addressed

**Findings & dispositions** (all RRs linked to TKT-0C57FS):

| RR | Severity | Finding | Disposition |
|---|---|---|---|
| RR-U4XVCI | critical | enum_values ledger entries wedged every GC tick forever | FIXED: enum drift never ledgered + unknown kinds dropped-and-warned; regression test |
| RR-FTY3QA | significant | cardinality unset↔0 min misclassified as needs-migration | FIXED: effective-value semantics; 6-case test |
| RR-X3EQHY | significant | runaway Lua script hangs the run | FIXED: LState bound to ctx; interruption test |
| RR-5YLNG7 | significant | free-edge marker rebase drops store divergence | ANALYZED SAFE: ledger prunes against live schema, never marker; divergence re-classified next evaluation; fails toward retention; documented in resolve.go |
| RR-69X625 | significant | endpoint deltas "fail after data rewrite" | ANALYZED: Resolve plans (incl. tail check) BEFORE any write; residue is a gen UX limitation, documented |
| RR-PX9LST | significant | Scan ledgered managed properties on entities | FIXED: underscore namespace skipped in both loops; test |
| RR-Z7JWJ6 | significant | GC deleted data on failed version capture | FIXED: capture failures are hard errors before every destructive delete; test |
| RR-H16NMN | significant | marker RMW race gate-vs-runner | FIXED amplifier (rename conflict no longer mutates) + race degrades to idempotent re-plan; last-write-wins semantics documented; lock named as upgrade |
| RR-PMZWS0 | minor | batch of 7 minors/nits | 5 fixed, 2 deferred with documented reasons (unreachable panic; deliberate interval-first tick) |

Reviewer-verified clean areas: script path validation, hash integrity,
generated-YAML injection safety (quoteYAML + draft re-parse), deletions never
emitted live, convert never destroys unconvertible data, forEachEntity Tx
batching, GC gate interlock, goroutine lifecycle, fsstore relocation fix.

## Acceptance Verification (PLAN-OX2A9U AC1–AC13)

1. Cosmetic edits don't demand migration — PASS (TestShapeProjectionHash_CosmeticEditsDoNotMoveIt, 16 cases incl. defaults-additive and id prefixes excluded)
2. Additive auto-adopt — PASS (TestGate_AdditiveAdoptsSilently + e2e bootstrap)
3. Drift adopt + rename warning — PASS (TestGate_DriftAdoptsAndRecordsLedger, TestCompareShapes_PossiblePropertyRename + e2e observed notice)
4. Needs-migration warns, writes stay soft — PASS (TestGate_NeedsMigrationDoesNotAdopt + e2e; entitymanager untouched)
5. gen GUESS/TODO drafts, deletions commented, projection embedded+hash-checked — PASS (generate tests + parse integrity negative test + e2e draft)
6. data dry-run/apply/crash-rerun/audit/marker — PASS (run_test.go incl. TestRunner_ReRunAfterPartialApplyIsIdempotent + e2e evidence in IMPL-NPFVB8)
7. Free-edge chain bridging — PASS (TestResolve_ChainAndFreeEdges, 7 subtests)
8. Lua pure transform incl. unknown_type — PASS (TestLuaStep_RunsOnUnknownTypeEntities)
9. GC grace/skip/prune/audit/capture — PASS (gc_test.go + review regression tests)
10. Multi-tenant independence — PASS by construction (state.KV per schema); dedicated two-schema pg test deferred to the pg CI matrix (marker/ledger are per-store state, exercised via statetest-conformant KV)
11. fsstore no orphan file + destructive capture — PASS (TestPersistence_TypeChangeLeavesNoOrphanFile verified red-without-fix; capture via failing-capture test; pg suite green with -race)
12. Gate re-evaluation — PASS (TestGate_ReEvaluateOnReloadUpdatesVerdict); note: no live server metamodel reload exists today (RR-FURO8P resolution)
13. All backends green — PASS (fs/mem in CI matrix, pg locally with -race)

## Verification

- [x] PR: (created after this checklist — see ticket)
- [x] All review-responses resolved (17 total: 8 design + 9 code; zero open critical/significant)
