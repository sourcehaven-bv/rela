---
id: REV-3G69SB
type: review-checklist
title: 'Review: Typed state references and the store contract (Step 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (per PR: `go test ./...`; PR-B also `just test-postgres` — DB-gated race suite against a locally provisioned postgres; 2026-08-20)
- [x] Lint clean (`just lint` 0 issues; `just plimsoll`; `just arch-lint`; `go build -tags postgres`)
- [x] Coverage maintained (`just coverage-check`: floors + total 78.1–78.2% PASS across the stack)

## Code Review

- [x] Run `/code-review` command (three times — once per PR of the stack; PR-B's reviewer additionally reproduced races and migration failure modes against a live database)
- [x] All critical review-responses addressed (4 across the stack: RR-LG8QUB, RR-ZI5ZLB, RR-HRC7JC, RR-7081BE — all fixed with pins)
- [x] All significant review-responses addressed (11 across design review + three code reviews — all fixed; see list)
- [x] Self-reviewed the diff for unrelated changes (each PR code-only; paperwork lands with this closing PR per the multi-PR discipline)

**Review Responses:**

Design review: RR-AUF5ZC, RR-IVIPQA (answered same-day by design-doc
amendments), RR-R6G2VM, RR-8U1PE2 — all addressed. PR-A: RR-LG8QUB (critical),
RR-5GI8XO, RR-2Y3L58, RR-QOJTMJ, RR-NTKQVY, RR-EPVQJB (significant), RR-TV1DM6
(minors) — addressed; RR-GSOQY1 (minor, O(n) family scans) deferred as a
follow-up optimization. PR-B: RR-ZI5ZLB, RR-HRC7JC, RR-7081BE (critical),
RR-0O6030, RR-HVUBS7 (significant), RR-L2PXEH (minors) — all addressed. PR-C:
RR-BYK5TO (significants), RR-RACANY (minors) — all addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (pointerless byte-compat) — PASS: zero-value semantics + full suites green + manual e2e (state files on disk, default reads identical).
- AC2 (pair addressing, bare id = default) — PASS: storetest AddressByPair on all three backends.
- AC3 (cascades over families) — PASS: storetest Delete/RenameCascadesTheFamily, unconditional per backend since PR-B.
- AC4 (pointer matching in BOTH implementations) — PASS: RelationTails incl. the fs indexed-query case; pg relationWhere predicate.
- AC5 (AllStates raw truth vs default) — PASS: QueryScope conformance case.
- AC6 (per-state events; pg feed codec round-trip) — PASS: EventsFirePerState (bounded blocking receive); feed/tombstone/catch-up codec paths.
- AC7 (codec grammar + round-trips + store opacity) — PASS: pointer_test.go tables + PointerIsOpaqueToTheStore.
- AC8 (detection: finding + warning, never refusal) — PASS: TestCheckStates (+ tolerated disk shapes), appbuild warn tests, manual e2e.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (metamodel `scope:` docs in both trees)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-94U6CK

## Final Checks

- [x] Commit message explains the why, not just what (one per PR; PR bodies carry the contract reasoning)
- [x] No TODOs or FIXMEs left unaddressed (the TKT-DOFYR1-PR-B TODO markers were all deleted with PR-B as mandated; remaining TODOs reference future steps by ticket)
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (three-PR stack; each monitored to green)
- [x] All CI checks pass (PR-A #1386 and PR-B #1388: all 24 checks; PR-C monitored after this paperwork commit)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1389 (PR-C, closing); stack:
#1386 (PR-A), #1388 (PR-B), stacked on #1381 ← #1378
