---
id: PLAN-T0WFR
type: planning-checklist
title: 'Planning: Extract stubEntityManager to shared test helper package'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
- IN: New `internal/entitymanager/entitymanagertest` package exporting `PanicOnUse` (struct value, panicking method bodies for every method on `entitymanager.EntityManager`).
- IN: Replace `docTestStubEM` (`internal/dataentry/document_script_test.go`) and `stubEntityManager` (`internal/script/executor_test.go`) with the shared type.
- IN: Add the new package to the arch-lint `exclude` list (mirroring `internal/store/storetest`).
- OUT: A more capable fake (recorder, scripted responses) — separate ticket if/when needed.

**Acceptance Criteria:**
1. New package compiles and is consumed by both test files. Verified by `just test`.
2. Existing tests in `internal/dataentry` and `internal/script` still pass unchanged.
3. `just arch-lint` passes.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**
- `internal/store/storetest` is the closest analogue: a test-only sibling package next to the production package. Same naming convention applied: `internal/entitymanager/entitymanagertest`.
- No external library — the interface lives in this repo, so a hand-rolled stub is correct.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- Create `internal/entitymanager/entitymanagertest/panic.go` with `type PanicOnUse struct{}` and one method per interface entry, each calling `panic("entitymanagertest.PanicOnUse.<Method>: not expected in this test")`.
- Compile-time assertion: `var _ entitymanager.EntityManager = PanicOnUse{}` so adding a method to the interface fails to build the helper, surfacing the gap once instead of in every consumer.
- Update both test files to import the new package and replace the stub type references.

**Files to modify:**
- (new) `internal/entitymanager/entitymanagertest/panic.go`
- `internal/dataentry/document_script_test.go`
- `internal/script/executor_test.go`
- `.go-arch-lint.yml`

## Security Considerations

- [x] ~~Input sources identified~~ (N/A: test-only helper, no inputs)
- [x] ~~Input validation approach defined~~ (N/A: test-only helper)
- [x] ~~Security-sensitive operations identified~~ (N/A: test-only helper)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: panics in tests only)

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- Existing tests in `internal/dataentry` and `internal/script` exercise the call sites where `PanicOnUse` is wired into `lua.WriteDeps`. They cover the read-path; if any test accidentally reaches a mutation method, it panics — the desired behaviour.

**Edge Cases:**
- Interface evolution: covered by `var _ entitymanager.EntityManager = PanicOnUse{}` compile-time assertion.

**Negative Tests:**
- Not introducing new tests for the helper itself; its contract (panic on use) is exercised by the absence of panics in the existing suites.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl): **s**

**Risks:**
- Low. Mechanical extraction of a duplicated test fixture. Mitigated by the compile-time interface assertion and existing test coverage.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal refactor, test-only)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor, not enhancement)

**Documentation Impact:**
- N/A — Internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: trivial mechanical extraction; scope and approach already specified in the ticket)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None.
