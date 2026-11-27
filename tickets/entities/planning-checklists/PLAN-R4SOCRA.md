---
id: PLAN-R4SOCRA
type: planning-checklist
title: 'Planning: Reduce Go and TypeScript code size'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: dead Go code, `sort` to `slices`/`cmp`, `go fix` modernizers, table-driven
tests, unused frontend files/exports/dependencies, props destructure,
`defineModel`, value-mapping switches to lookups.

Out: behavior changes, new abstractions, sorts that need new compare helpers,
`third_party/`, the `omitzero` fixer (changes JSON output).

**Acceptance Criteria:**

1. Build, vet and lint pass for every build tag: run them per tag set.
2. Test suites pass: `go test` per tag set, `npm run test:run`.
3. No test named in docs or CLAUDE.md is removed: grep before merging tests.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: mechanical refactor)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: tool-driven rewrites)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: no domain change)

**Research Doc:** N/A

**Existing Solutions:** `deadcode`, `go fix` modernizers, `knip`, Vue 3.5
props destructure and `defineModel`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Run the tools, review each rewrite, revert rewrites
that read worse. A sort with a custom less helper stays on `sort.Slice`;
rewriting it would need a new compare helper.

**Files to modify:** Go and frontend sources and tests across the tree.

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: no new inputs)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no new inputs)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: no behavior change)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: error paths unchanged)

**Input Sources & Validation:** None added.

**Security-Sensitive Operations:** None changed.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: existing tests cover them)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: existing suites)

**Test Scenarios:** The existing suites are the test of a refactor.

**Edge Cases:** Sort stability and tie-breaks; `defineModel` same-value no-emit.

**Negative Tests:** Existing.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: no security surface)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A comparator rewrite that changes order; a removed export used
dynamically. Mitigation: full test suites and code review.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore)

**Documentation Impact:** N/A, internal change.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: tool-driven refactor with no design change)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
