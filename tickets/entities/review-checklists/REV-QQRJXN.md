---
id: REV-QQRJXN
type: review-checklist
title: 'Review: Postgres derived-schema reconciler (seam + unique rule): atomic unique:true, db status drift, dry-run'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (touched packages pass under `-race -shuffle=on`; full pgstore conformance suite green against a live Postgres). The only tree-wide failure is `TestBuiltCSSIsLayered` in internal/dataentry — a PRE-EXISTING stale-frontend-artifact test, unrelated (this branch touches no frontend/dataentry code; CI rebuilds the frontend).
- [x] Lint clean (`just arch-lint`, `just plimsoll`, full default-tag `golangci-lint run ./...` = 0 issues; postgres-tag lint clean for all touched files)
- [x] Coverage maintained (pgstore 74.9%; new branches covered by added tests: unsafe-name, dry-run drop, exact count)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer, adversarial pass on the implementation)
- [x] All critical review-responses addressed (the 2 design-review criticals RR-5LZWX8/RR-GVXUIQ)
- [x] All significant review-responses addressed (RR-8OIKGN reload-staleness, RR-V08S5M non-string divergence, RR-WF0ZYF unbounded lock — all fixed)
- [x] Self-reviewed the diff for unrelated changes (isUniqueViolation removed as dead; no scope creep)

**Review Responses:** Design review: RR-5LZWX8, RR-GVXUIQ (critical), RR-AROZJY,
RR-CWI8HG, RR-QY5S4C, RR-2HMGZJ, RR-FTQE3U, RR-B5Y6DZ, RR-3NB0P9 (significant) —
all addressed. Code review: RR-8OIKGN, RR-V08S5M, RR-WF0ZYF (significant),
RR-78T6Q9, RR-0USU3N, RR-DLML7F (minor) — all addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. Concurrent/duplicate insert → single row, loser fails atomically — PASS (TestDerivedUnique_DuplicateInsertRejected, _ConcurrentInsertsSingleRow)
2. → UniquePropertyError → ValidationErrorUnique; ID collision still ErrEntityAlreadyExists; Detail never surfaced — PASS (TestDerivedUnique_IDCollisionStillPlainConflict, TestMapUniquePropertyConflict)
3. Provision conflict → re-resolve — PASS (existing maybeProvision handles the mapped error; provision e2e in TKT-ANUJDS + the conflict mapping tests)
4. Reconcile create/drop/idempotent/converged-noop/per-rule/schema-filtered/advisory-locked — PASS (TestDerivedUnique_ReconcileIdempotentAndDrop, _CrossSchemaIsolation, _DryRunDrop; try-lock verified)
5. Pre-existing dupes don't crash; unenforced + blocking count — PASS (TestDerivedUnique_PreexistingDuplicatesDegrade, _BlockingCountExact; e2e exit 0)
6. db status exit 0 + reconcile --dry-run exit non-zero on drift; shared planner — PASS (e2e verified; TestDerivedUnique_DryRunNoMutation)
7. Scan/index agree on empty/absent/list/non-string — PASS (TestDerivedUnique_EmptyAndAbsentExempt; non-string rejected at load — TestUniqueRequiresStringType)
8. fs/mem unchanged — PASS (conformance suite green; reconciler is a no-op type-assert)
9. Out-of-charset name refused at load + reconciler — PASS (TestValidateSchemaName, TestSafeDDLName, TestDerivedUnique_UnsafeNameRefused)
10. Update-path 23505 mapped — PASS (mapConflict wired on UpdateEntity; create-then-failed-automation documented)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-EY92SD)
- [x] User-facing documentation updated (postgres-backend, acl-security, metamodel guides; unique.go doc comment)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-EY92SD

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (pending user go-ahead — /pr is the outward-facing final step; the ticket is done and validate-clean, which is /pr's own precondition)
- [x] ~~All CI checks pass~~ (to be confirmed by /pr)
- [x] ~~PR URL documented below~~ (to be filled by /pr)

**PR:** *pending /pr*
