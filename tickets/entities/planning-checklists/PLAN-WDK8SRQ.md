---
id: PLAN-WDK8SRQ
type: planning-checklist
title: 'Planning: Unset properties from a flows.yaml trigger'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `set.unset` on flows.yaml triggers, applied on update, create,
copy and cascade writes; load-time validation; docs. Out: unsetting relations,
Lua changes.

**Acceptance Criteria:**
1. Reopen clears `completed_on` (engine test and entitymanager test).
2. A property both set and unset, an unknown, required, computed, file or external-ref property fails the load (parse/schema tests).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small extension of an existing output)
- [x] ~~Searched for existing libraries~~ (N/A: internal config surface)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal config surface)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `set` output (parse.go decodeOutput, compile.go
renderOutput); `entity.Patch.MetaUnset` semantics in entitymanager.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See the plan: `rawSet.Unset`, `Action.Unset`,
`Result.PropertiesUnset`, `Result.Writes()` returning
`triggers.PropertyWrites{Set, Unset}` used by every consumer. The removal folds
into the write the trigger fires on (update: the in-flight entity the manager
already merged against the raw row; create: the CAS re-read in
writeAutomationProperties). Rejected: a separate `PatchEntity` call from the
engine, which would re-run automations and add a second write.

**Files to modify:**
internal/flowcfg/triggers/{parse,types,compile,engine,schemacheck}.go,
internal/entitymanager/{manager,cas,copy_automation,cascadehost}.go,
internal/autocascade/{runner,host}.go, docs-project flows guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** flows.yaml (operator config). Names must be
declared properties of every `on.entity` type (allowlist).

**Security-Sensitive Operations:** A system write, like `set`. File and
external-ref properties refused, as for `set`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** engine test `TestSetUnset_ClearsCompletedOnReopen`;
entitymanager update/patch/create tests.

**Edge Cases:** unset of an absent property is a no-op; caller patch sets the
property while the trigger unsets it (trigger wins, as for set).

**Negative Tests:** parse errors for empty set, duplicates, set+unset overlap;
schema errors for unknown/required/file/external-ref/computed.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A consumer forgets the unset: mitigated by `Result.HasWrites()` at
every site and a test per path. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs-project flows guide (generates docs/flows.md).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Required property unset refused at load
(significant, addressed). Trigger wins over the caller's patch, tested
(significant, addressed). One `PropertyWrites` shape and `HasWrites()` (minor,
addressed). Unset of absent key is a no-op (minor, addressed).
