---
id: REV-B7UX67
type: review-checklist
title: 'Review: Machine-aware status control: surface _transitions on the wire + SPA performable-transition UI + entry-locked create field'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — Go `go test ./...` clean; frontend `npm run test:run` 1342 pass
- [x] Lint clean — `just lint` 0 issues; `just arch-lint` OK; eslint 0 errors on changed files
- [x] Coverage maintained — `just coverage-check` PASS (total 76.2%, all floors met)

## Code Review

- [x] Ran `/code-review` (cranky-code-reviewer agent)
- [x] All critical review-responses addressed (RR-DENG8U, RR-C3OJ33)
- [x] All significant review-responses addressed (RR-NI145G)
- [x] Self-reviewed the diff for unrelated changes — reverted 117 files of stray
Prettier reformatting that `npm run format` swept in; final diff is scoped to
the ticket only

**Review Responses:** RR-DENG8U (critical, addressed), RR-C3OJ33 (critical,
addressed), RR-NI145G (significant, addressed), RR-NN5414 (minor, addressed),
RR-SG2I1G (minor, addressed), RR-IQZ62Y (nit, addressed). No open
critical/significant.

## Acceptance Verification

- [x] Each acceptance criterion tested (see planning PLAN-QQLK3F + impl IMPL-ZPQHEW)

**Acceptance Status:**
- AC1 (wire `_transitions`, machine-only, absent otherwise): PASS —
TestTransitionsWire_GETCarriesTransitions / _AbsentWithoutTransitionResolver;
hidden-field variant TestTransitionsWire_HiddenMachineFieldOmitted.
- AC2 (SPA renders performable moves, falls back for non-machine): PASS —
FieldRenderer.test.ts routing + StatusControl.test.ts (only-allowed, labels).
- AC3 (select commits atomic PATCH, 403/422 surfaces): PASS — StatusControl emits
target → useAutoSave field-save; existing structured-error path unchanged.
- AC4 (create locked to initial, non-editable): PASS —
TestTransitionsWire_CreateLocksMachineField + _CreateLockSkipsHiddenField;
adoptLockedFieldValues unit tests.
- AC5 (label fallback chain): PASS — TestPerformable_SurfacesLabel;
StatusControl.test.ts raw-value fallback.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-V9830X)
- [x] User-facing documentation updated (api-reference.md, metamodel.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-V9830X

## Final Checks

- [x] Commit messages explain the why (feat + fix(review) with RR references)
- [x] No TODOs/FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Ran `/pr` — PR #1156 created and CI monitored
- [x] All CI checks pass — 0 failing on the full matrix (Test, Frontend, E2E, Cross-Compile ×8, Postgres, Fuzz, Lint, Architecture, God-object, Markdown, Rela Tickets, CodeQL); `mergeable: MERGEABLE` (BLOCKED only on required human review)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1156
