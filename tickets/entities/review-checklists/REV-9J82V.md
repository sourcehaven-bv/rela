---
id: REV-9J82V
type: review-checklist
title: 'Review: Declarative status/enum state machines: transitions on CustomType with ACL-permission guards + predicate preconditions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` + `-race` on statemachine/entitymanager/appbuild)
- [x] Lint clean (`golangci-lint run ./...` → 0 issues; `just arch-lint`; `just plimsoll`)
- [x] Coverage maintained (`just coverage-check` PASS; statemachine 89%)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer) + `/crit` (approved, 0 comments over 63 files)
- [x] All critical review-responses addressed (RR-NB135, RR-HETEE, RR-FPZJX)
- [x] All significant review-responses addressed (RR-UJPW4, RR-EU8GJ, RR-1SMG4, RR-NODYR, RR-VB2DE, RR-UOBUC)
- [x] Self-reviewed the diff for unrelated changes (only the transition feature + the mechanical Transitions-field additions to existing entitymanager test Deps)

**Review Responses:** Design review — RR-FPZJX, RR-UJPW4, RR-EU8GJ, RR-1SMG4,
RR-NGBMT, RR-SGQO3. Code review — RR-NB135, RR-HETEE, RR-NODYR, RR-VB2DE,
RR-UOBUC, RR-F30CZ. All 12 `addressed`.

## Acceptance Verification

- [x] Each acceptance criterion tested (see planning PLAN-5BYO6 + implementation IMPL-CP7L2 evidence)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 declare/inherit machine — PASS (`TestCompile_WellFormed`)
- AC2 update legality 422 — PASS (`TestEnforceUpdate_Legality`, `TestTransition_IllegalMoveIs422`)
- AC3 served guard 403 subject-aware, CLI inert — PASS (`TestTransition_GuardDeniedIs403`, `TestRequest_HoldsPermissionForEntity_SubjectScoped`, guard-inert case)
- AC4 when precondition 422 w/ graph funcs — PASS (`TestEnforceUpdate_When`, `_EntityValueBinding`)
- AC5 create defaults initial; non-initial 422 — PASS (`TestTransition_IllegalEntryOnCreateIs422`, `_IllegalEntry_DoesNotPersist`)
- AC6 load-time rejects dangling — PASS (`TestCompile_Errors`, `_RejectsUndeclaredDefaultEntry`)
- AC7 inline enums documented unguarded — PASS (godoc named-type-only; `TestCompile_RejectsTransitionsOnListProperty`)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (godoc + design doc; live metamodel-reference deferred until a project adopts the feature — see DOCS-STOGZ)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-STOGZ

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (empty enforcer = no-op; additive; nil-rejected wiring)

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending /pr -->
