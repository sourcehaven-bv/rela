---
id: IMPL-CP7L2
type: implementation-checklist
title: 'Implementation: Declarative status/enum state machines: transitions on CustomType with ACL-permission guards + predicate preconditions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (internal/statemachine: Compile validation, Enforce legality/guard/when, create-entry; internal/acl: subject-aware permission)
- [x] Integration tests written (entitymanager/transition_test.go: full Create/Update/ApplyEntity flow → 422/403 mapping, no-orphan-on-rejection)
- [x] Happy path implemented (legal transitions pass; unconstrained enums unaffected)
- [x] Edge cases handled (illegal skip/backward, guard denied, precondition false, illegal entry, clear-to-empty, whitespace, list-property rejection, Default-entry validation)
- [x] Error handling in place (typed sentinels ErrIllegalTransition/ErrGuardDenied/ErrPreconditionFailed/ErrIllegalEntry + GuardError; mapped to 422/403 at the boundary)

## Test Quality

- [x] Using fixture builders (snapshotMeta(), ent(), fakeGuard/fakeLookup helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end in a running app~~ (N/A: enforcement is exercised end-to-end by the integration tests through the real entitymanager write path — Create/Update/ApplyEntity — with real Compile output; no UI surface in this ticket's scope)
- [x] Each acceptance criterion verified with a test scenario (see evidence)
- [x] Edge cases verified via table-driven tests

**Verification Evidence:**

All 7 acceptance criteria covered by automated tests:
- AC1 (declare transitions/initial, inherited by property): `TestCompile_WellFormed`
- AC2 (update rejected 422 unless declared edge): `TestEnforceUpdate_Legality`, `TestTransition_IllegalMoveIs422`
- AC3 (served guard → 403 subject-aware; RuleID=permission; CLI inert): `TestTransition_GuardDeniedIs403`, `TestRequest_HoldsPermissionForEntity_SubjectScoped`, `TestEnforceUpdate_Guard` (inert case)
- AC4 (when → 422 with graph host funcs): `TestEnforceUpdate_When`, `TestEnforceUpdate_When_EntityValueBinding`
- AC5 (create defaults to initial; non-initial → 422): `TestTransition_IllegalEntryOnCreateIs422`, `TestTransition_LegalEntryOnCreatePasses`
- AC6 (load-time rejects dangling from/to/initial): `TestCompile_Errors`, `TestCompile_RejectsUndeclaredDefaultEntry`
- AC7 (inline enums documented unguarded): named-type-only documented on `CustomType.Transitions` godoc; list-property rejected (`TestCompile_RejectsTransitionsOnListProperty`)

CI: full `go test ./...`, `-race`, `golangci-lint ./...` (0 issues), `just
arch-lint`, `just plimsoll`, `just coverage-check` (statemachine 89%) — all
green.

## Quality

- [x] Code follows project patterns (consumer-side interfaces `Guard`/`GraphLookup`/`TransitionEnforcer`; required Deps collaborator nil-rejected; capability split; fail-fast constructor)
- [x] Checked for DRY (grantsPermission extracted from holds/HoldsPermissionForEntity; CompileTransitions shared by prod + fixture wiring)
- [x] No security issues introduced (guard fails closed under active policy; subject-aware resolution; delegate-gate self-promotion documented)
- [x] No silent failures (errors surfaced as typed sentinels and returned/mapped)
- [x] No debug code left behind
