---
id: PLAN-7BFKU1T
type: planning-checklist
title: 'Planning: Copies fire automations; fields: all with set: overrides'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: run created/updated automations after a copy commits; `on.copy:`
trigger key; `set:` copy key beside `fields: all`; docs. Out: a `copy` variable
in the predicate env (the trigger key covers it and is checked at load);
surfacing automation output in the copy HTTP response.

**Acceptance Criteria:**
1. A copy that creates the face runs `created` automations (TestCopyRunsAutomations).
2. A copy that overwrites runs `updated` automations, background actions included (TestCopyRunsAutomations).
3. `on.copy:` narrows to named copies; unknown name is a load error (TestCopyTrigger, TestValidateAutomationCopies).
4. `fields: all` + `set:` stamps values (TestCopyRunsAutomations, TestValidateCopies).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change on an existing seam)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal wiring)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal wiring)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** The create path's `createFollowUps` (manager.go) runs
automations after a durable write, writes their properties with
`writeAutomationProperties` and runs the cascade through `m.gated()`. The copy
path follows it. `keep:` (TKT-2Z2IA6) is the precedent for a sibling key beside
`fields: all`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `copyState` splits into `writeCopy` (unchanged, under
the attachment lock) and `runCopyAutomations` (after the lock is released).
`automation.Event` gains `Copy`, `Trigger` gains `Copy`; `matchesCopy` applies
the scope. `CopyDef` gains `Set`, applied in `buildCopyTarget` with the
field-map grammar. Alternative rejected: a `copy` variable in predicate
`condition:`, which would need a new compile profile and could not be checked
against `copies:` at load.

**Files to modify:** internal/entitymanager/copy.go, copy_automation.go;
internal/automation/types.go, engine.go; internal/metamodel/types.go, copies.go,
automationcopy.go, loader.go; internal/configedit/allowlist.go;
frontend/src/configure/models.ts; docs-project guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Only schema.yaml (operator config). `on.copy`
names and `set:` properties are checked at load. A request still names a copy
only by name.

**Security-Sensitive Operations:** Automation writes after a copy are automation
output, not the caller's writes, as on create. The cascade runs through the
gated mutator, so elevation never reaches nested writes. `set:` refuses file
properties and external refs at load.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: entitymanager tests run the real engine, cascade and memstore)

**Test Scenarios:** see Acceptance Criteria.

**Edge Cases:** A copy that changes no value still fires a copy-only trigger;
rename re-runs a copy trigger; a manager without automations copies as before.

**Negative Tests:** Unknown copy name, excluded entity or face, relation event
beside `copy:`; `set:` without `fields: all`, on a kept, file, external-ref or
undeclared property.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Existing `updated`/`created` automations now also fire on copies.
That is the requested behaviour; `faces:` and `copy:` narrow it. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md - New metamodel features
- [x] docs/content-states.md - copies

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change on an existing seam; covered by the code review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
