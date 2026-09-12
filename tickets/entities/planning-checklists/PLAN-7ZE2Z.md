---
id: PLAN-7ZE2Z
type: planning-checklist
title: 'Planning: Add native relation-cardinality support to validation rules (relations: block on ValidationRule)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: native `relations:` block (min/max/where) on ValidationRule;
evaluation in internal/validation; strict loader rejecting unknown
validation-rule keys; migrate the 14 gates off Lua and delete the stopgap. OUT:
changing any gate's semantics (behaviour must match the Lua baseline).

**Acceptance Criteria:**
1. `relations:` parsed + evaluated (min/max/where) — unit tests in
internal/validation and a loader parse test.
2. Loader rejects an unknown validation-rule key (regression for the root cause).
3. 14 gates declarative again; `rela validate` output identical to the Lua
baseline (per-gate count parity harness).

## Research

- [x] ~~/research~~ (N/A: scope fixed by TKT-IFHO2L follow-up)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**
- Reused the existing filter predicate machinery (filter.ParseAll / MatchAll)
for `where:` target-property matching — identical to how when/then work.
- Mirrored the existing top-level `validTopLevelKeys` +
TestValidTopLevelKeysMatchStruct parity pattern for the new per-rule whitelist.
- RelationDef already has MinOutgoing/MaxOutgoing on relation *types*; this adds
the same idea scoped to a validation rule with target filtering.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Add `Relations map[string]RelationConstraint`
(Where/Min/Max) to ValidationRule. Evaluate in validation.checkEntityAgainstRule
alongside content: count outgoing relations of the type whose target matches the
where-filters, assert min/max. Read relations via a new
lua.ReadDeps.OutgoingRelations helper so validation does not import
internal/store (arch-lint boundary). Strict loader:
checkUnknownValidationRuleKeys walks the validations list.

**Alternatives considered:**
- Do the check in internal/analysis (has Store directly) — rejected: the
per-rule dispatch lives in internal/validation; splitting it there would
fragment rule evaluation.
- Import internal/store into validation — rejected by arch-lint; used a
ReadDeps helper instead (consumer-side, no new boundary).

**Files to modify:**
- internal/metamodel/types.go, loader.go (+ tests)
- internal/lua/deps.go
- internal/validation/validation.go (+ tests)
- tickets/metamodel.yaml; delete tickets/validations/require-relation-count.lua

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Rule config is trusted metamodel YAML. The new strict-loader is an ALLOWLIST
(validValidationRuleKeys) — the preferred posture. Filter/store errors degrade
the check to a no-op rather than a spurious violation (matches when/then).

**Security-Sensitive Operations:** none (read-only graph traversal).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- min: missing / not-done / done targets; when-not-matching skip.
- max:0 response gates: none / open-critical / addressed / severity-mismatch.
- loader: unknown key rejected (names key + rule); valid relations parsed.
- parity: per-gate violation counts identical to the Lua baseline (harness).

**Edge Cases:**
- No store wired (unit service) → no-op, no panic (explicit test).
- Dangling relation target (GetEntity error) → not counted.
- where empty → count all targets of the type.

**Negative Tests:**
- `relationz` typo → load fails loudly (TestParse_UnknownValidationRuleKeyRejected).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Behaviour drift vs Lua stopgap → mitigated by a per-gate count-parity harness
(identical before/after).
- Strict loader falsely rejecting existing metamodels → scanned every
metamodel.yaml in the repo; no false rejections. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md — the `relations:` validation block is a new documented
metamodel feature (enhancement); will update in the docs checklist.

## Design Review

- [x] ~~Run /design-review~~ (N/A: approach agreed with maintainer; direct follow-up of TKT-IFHO2L)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
