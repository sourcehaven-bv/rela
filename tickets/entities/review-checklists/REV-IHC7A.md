<!-- @managed: claude-workflow v1 -->
---
id: REV-IHC7A
type: review-checklist
title: 'Review: Per-channel debounce + checkbox-toggle to useAutoSave'
status: done
---

## Automated Checks

- [x] All tests pass — local `npm run test:run`: 926/926; CI Test job pass (2m16s)
- [x] Lint clean — local `npm run lint`: 0 errors; CI Lint job pass (2m1s)
- [x] ~~Coverage maintained~~ (N/A: frontend tracks per-file 100% ratchet via the frontend coverage step, which the CI Frontend job validated and passed)

## Code Review

- [x] Run `/code-review` command — design-review pass captured 10 RR-FA1\* findings (cranky design-reviewer agent) before implementation; no additional findings on the implementation diff
- [x] All critical review-responses addressed — 0 critical
- [x] All significant review-responses addressed — RR-FA1A (runtime assert), RR-FA1B (baseline preserved), RR-FA1F (e2e timing verified by reading spec)
- [x] Self-reviewed the diff for unrelated changes — diff is the `useAutoSave` API extension + `EntityDetail` refactor + tests; no churn

**Review Responses:** RR-FA1A, RR-FA1B, RR-FA1C, RR-FA1D, RR-FA1E, RR-FA1F, RR-FA1G, RR-FA1H, RR-FA1I, RR-FA1J

## Acceptance Verification

- [x] Each acceptance criterion tested — see PLAN-IHC7A ACs 1-10
- [x] Test evidence documented in implementation checklist — see IMPL-IHC7A Verification Evidence

**Acceptance Status:**

- AC 1 (per-channel debounce): PASS — `useAutoSave.test.ts` "IHC7A AC1: fieldDebounceMs and contentDebounceMs fire independently" + legacy fallback
- AC 2 (initialServerSnapshot): PASS — "IHC7A AC2: initialServerSnapshot seeds the no-op baseline" + replace-on-recordSnapshot
- AC 3 (disabled channels): PASS — "IHC7A AC3: disabled channels throw" + merge-skips-apply + commit no-op
- AC 4 (EntityDetail refactor): PASS — e2e `checkboxes.spec.ts` passed unchanged on CI; `togglingIndices` removed; route-change guard via `pinEntityForFlush`
- AC 5-10: covered by unit + e2e suite as above

## Documentation (enhancements only)

Skip — internal refactor + composable extension; no user-facing docs affected.

## Final Checks

- [x] Commit message explains the why, not just what — feat commit body covers the channel-disable rationale + lineage note
- [x] No TODOs or FIXMEs left unaddressed — checked diff
- [x] Ready for another developer to use — `useAutoSave` API is documented via jsdoc on each new option (RR-FA1C, RR-FA1H)

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI — PR #912
- [x] All CI checks pass — all non-bookkeeping checks green; this checklist + ticket status flip resolves the `Rela Tickets` gate
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/912
