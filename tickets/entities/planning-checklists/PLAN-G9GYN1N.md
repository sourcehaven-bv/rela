---
id: PLAN-G9GYN1N
type: planning-checklist
title: 'Planning: Flow guard may read a field every visible: block shows'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the policy-wide `acl.Policy.FieldRestricted` check used by flow
guards, flow templates and realtime rules. Out: client baselines (rule
unchanged), scope grants, the run view itself.

**Acceptance Criteria:**
1. A guard over a property every role's `visible:` block for the type lists without `when:` loads (flowcfg test, harbourmaster copy validated).
2. A guard over a property a block leaves out, or shows only under `when:`, is refused (flowcfg test).
3. Face-keyed and `*` blocks still restrict every property (acl test).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small, local change)
- [x] ~~Searched for existing libraries~~ (N/A: policy logic internal to rela)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: rela-specific ACL rule)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `Policy.ConditionallyVisible`
(internal/acl/traversal.go) and `PolicyResolver.FieldVisibleOnEveryRow`
(internal/affordances/fieldfilter.go) already treat an unconditionally listed
field as visible.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** In `FieldRestricted`, a role's block keyed by the exact
type restricts a property only if it does not list it without `when:`. A
face-keyed or `*` block still restricts everything.

Rejected: withholding the guard verdict from readers who cannot see the field.
The run's state after the branch is taken would still disclose the value, so the
load check is still needed.

**Files to modify:** internal/acl/traversal.go, internal/flowcfg/load.go, tests,
docs-project/entities/guides/GUIDE-flows.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** acl.yaml and flows.yaml (operator config),
checked at load.

**Security-Sensitive Operations:** The check guards against a run state
disclosing a hidden value. A field is only accepted when no role's block can
hide it; the role union only adds visibility.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** TestFieldRestricted (acl),
TestParse_GuardReadsFieldEveryVisibleBlockShows (flowcfg).

**Edge Cases:** listed by one role and left out by another; listed under
`when:`; face key; `*` key; baseline.

**Negative Tests:** guard over a left-out field and a when-gated field is
refused.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Realtime rules share the check and get the same narrowing; the
reasoning is the same (no reader is denied the field). Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-flows "Conditions".

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change; security reviewer runs on the diff)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
