---
id: PLAN-P70DIM2
type: planning-checklist
title: 'Planning: Backend: validation: advisory|strict config that blocks on error-severity (DEC-OYV2AK)'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a per-deployment `validation: advisory|strict` setting
(`.rela/config.yaml`, overridden by `RELA_VALIDATION`); strict moves
required-missing, type-mismatch and invalid-value errors into the hard partition
on create, update, patch, restore, copy and conversion; 422 responses list each
failure in `errors[]`. Out: frontend rendering (TKT-1ARGYN), per-project mode,
the create dry run (stays advisory), the post-create automation write (row
already stored).

**Acceptance Criteria:**
1. Advisory mode is unchanged: soft errors are 200 + warnings (TestValidationMode_Create, existing suites).
2. Strict refuses required/type/enum failures with a ValidationError and stores nothing (TestValidationMode_Create, _Update, _StrictPatch).
3. Strict judges the post-automation state on update (TestValidationMode_Update "automation fills required").
4. The dry run stays advisory (TestValidationMode_DryRunStaysAdvisory).
5. The mode resolves from env, then config, then default; a bad value fails the boot (TestValidationModeFor).
6. 422 carries errors[] with JSON pointers (TestWriteWriteRefused_FieldErrors).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach fixed by DEC-OYV2AK)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal policy switch)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal policy switch)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `partitionValidationErrors`
(internal/entitymanager/validation.go) already splits hard from soft; DEC-OYV2AK
says to move the line there. `.rela/config.yaml` (internal/project/config.go) is
the existing per-checkout settings file.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `entitymanager.ValidationMode` on `Deps`;
`partitionValidationErrors(mode, errs)`. Pre-automation checks stay structural;
updateCore applies the mode to the final state. appbuild resolves the mode once
per SharedBase. dataentry `writeWriteRefused` fills `errors[]`. Rejected: tying
strict to the postgres build tag (DEC-OYV2AK); a `schema.yaml` key (the mode is
per deployment, not per schema).

**Files to modify:**
internal/entitymanager/{validation,manager,core,recreate,copy,conversion_flow,cascadehost}.go,
internal/appbuild/{appbuild,validationmode}.go, internal/project/config.go,
internal/dataentry/write_handler.go, docs-project guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The mode comes from operator config or env; only
`advisory`/`strict` are accepted, anything else fails the boot. Write payloads
are validated by the existing metamodel validator.

**Security-Sensitive Operations:** None new. `errors[]` details reuse the
value-free validation messages (BUG-197XPAU).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria; tests are table-driven against a
real memstore-backed Manager with automations.

**Edge Cases:** Automation filling a required property; env overriding config;
missing config file; property names needing JSON-pointer escaping.

**Negative Tests:** Unknown mode in env or config fails; strict refusals leave
the stored row unchanged.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A strict deployment cannot require a property only an on-create
automation sets (create persists first); documented with the template-default
workaround. Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/cli-reference.md - "Validation mode" under Project Configuration

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design fixed by accepted DEC-OYV2AK; the code review covers the implementation)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
