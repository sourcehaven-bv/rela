---
id: PLAN-5HCR16
type: planning-checklist
title: 'Planning: Copy option to keep named fields on the target face'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a `keep:` list on a copy definition. The copy leaves each named
property of the target face as it is. Load-time validation. Docs. Out: keeping
the body; `keep` beside a field mapping (unmapped fields are already kept
there).

**Acceptance Criteria:**
1. `fields: all` + `keep: [ondertekend]`: an existing target value of
`ondertekend` survives the copy, whatever the source holds.
2. When the target has no value, the field stays unset, even if the source has
one: `keep` means the copy never writes the field.
3. A kept file property keeps the target's own references, so its bytes stay
referenced and the post-copy sweep keeps them. The source-subset check does not
apply to a kept property.
4. Load errors: `keep` without `fields: all`; a kept name the target type does
not declare.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small, single option)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `keepTargetExternalRefs` in
internal/entitymanager/copy.go already keeps the target's value of a property;
`keep` generalises it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `CopyDef.Keep []string` (`yaml:"keep"`), decoded in
`UnmarshalYAML`. `validateCopy` refuses `keep` without `fields: all` and
undeclared names. `buildCopyTarget` restores each kept property from the
existing target (or deletes it) after the full replace; `confineFileValues`
skips kept properties. Alternative rejected: "keep unless the source has a
value"; it makes the result depend on the source and would move file references
between faces.

**Files to modify:** internal/metamodel/types.go, copies.go;
internal/entitymanager/copy.go; docs/content-states.md; tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Operator-authored schema.yaml; names must be
declared target properties.

**Security-Sensitive Operations:** File references are capabilities. A kept
value is the target's own, so no new reference is minted; the subset check stays
on every written file property.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** entitymanager copy tests for criteria 1-3 (including the
kept file's bytes still present after a second adoption); metamodel load tests
for criterion 4.

**Edge Cases:** New target face (nothing to keep); kept property that is also an
external ref.

**Negative Tests:** Load errors in criterion 4.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** The shadow struct in UnmarshalYAML could drop the new field; pinned
by TestCopyDef_UnmarshalCoversEveryField. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/content-states.md (copy keys table and the
full-replace paragraph).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Self-review. Significant: file bytes and reference
counting for kept files (addressed in criterion 3). Minor: semantics when the
target has no value (addressed in criterion 2).
