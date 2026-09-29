---
id: PLAN-I784WB
type: planning-checklist
title: 'Planning: Computed properties: reject enum literals outside the enum at load'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: string literals that can become a computed enum property's value (a
constant root, the value branches of `and`/`or` selections), for inline enums
(`type: enum` + `values:`) and custom types with `values:`.

Out of scope: attribute branches (`or entity.other_enum`) whose value set could
differ; results of concatenation or host functions; custom-type `validations:`
(patterns etc.); enum checks for conditions and filters, where a literal outside
the enum is merely never equal.

**Acceptance Criteria:**
1. `computed: entity.m == 'a' and 'hgh' or 'low'` on an enum property with values `[low, medium, high]` fails `computed.Compile` with an error naming the entity, property and `"hgh"`.
2. The same for a literal root (`computed: "'hgh'"`), a nested branch, and a custom enum type.
3. A valid mapping, an attribute branch (`or entity.base`), a string property and a custom type without `values:` compile as before.
4. `rela validate` on a scratch project reports the error.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small, approach follows from TKT-WQJGPS)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal type check)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal type check)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Enum resolution mirrors write validation: `internal/metamodel/validation.go:386` (inline `Values` on `type: enum`) and `validateCustomTypeValue` (`meta.Types[pd.Type].Values`).
- Program introspection follows `Program.Attributes`/`Traversals` in `internal/predicate/program.go`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `internal/predicate/program.go`: `func (p *Program) ResultLiterals() []Value` walks from the root: a `constNode` contributes its value; a selecting `logicalNode` recurses into its value operands (rhs of `and`, both of `or`); every other node contributes nothing. Pure introspection; no semantic change.
2. `internal/computed/computed.go` `compileProperty`: resolve the enum values (`pd.Values` when `pd.Type == enum`, else `meta.Types[pd.Type].Values` when non-empty). For each `predicate.String` in `prog.ResultLiterals()` not in the set, return the problem `entity %q property %q computed: %q is not one of the enum values [...]`. The existing "invalid definitions" aggregation reports it.

Alternative rejected: typing enums as a distinct predicate type. Larger change
(every comparison, coercion and the browser engine) for the same load-time
benefit.

**Files to modify:** `internal/predicate/program.go`,
`internal/predicate/selection_test.go`, `internal/computed/computed.go`,
`internal/computed/computed_test.go`, the computed-property section of
`docs-project/entities/guides/GUIDE-metamodel.md` (then `just docs`).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** operator-authored `schema.yaml`; the check is an
allowlist against the declared values. Config is not secret, so naming the
literal and the values in the error is fine.

**Security-Sensitive Operations:** none.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- `predicate`: `ResultLiterals` table test: literal root, selection chain, nested selection, condition literals excluded (`entity.m == 'x'`), concatenation and attributes excluded.
- `computed`: table test for AC1-AC3 through `computed.Compile`.
- Manual: `rela validate` on a scratch project (AC4).

**Edge Cases:** the condition's own literals (`entity.m == 'passkey'`) are not
result literals; a number literal on an enum property is already a type error;
`nil` root yields no string.

**Negative Tests:** AC1, AC2.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** an existing schema with a latent bad literal now fails to load. That
is the intent; the error names the fix. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md (via GUIDE-metamodel.md): one sentence under computed properties.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none critical or significant. Considered: custom
types with `values:` and `validations:` (values checked, validations out of
scope); list enums (computed lists are refused already).
