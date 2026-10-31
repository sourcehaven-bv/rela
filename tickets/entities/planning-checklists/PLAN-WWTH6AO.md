---
id: PLAN-WWTH6AO
type: planning-checklist
title: 'Planning: Break the Configure api/configure and stores/configure import cycles'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: batch 1 of archplan handover ho-22c49f92 (finding
`structure.tangle-b1bf2e78506f`). Move `TreeScalar`/`TreeMap`/`TreeValue` from
`frontend/src/configure/tree.ts` to `frontend/src/api/configure.ts`, and move
`frontend/src/stores/configDraft.ts` plus its test into
`frontend/src/configure/`.

Out of scope: the later batches of the handover, any change to the store's
behaviour, its Pinia id or its localStorage key.

**Acceptance Criteria:**

1. `grep` finds no import of `@/configure` under `frontend/src/api`.
2. `grep` finds no import of `@/stores` under `frontend/src/configure`, and no
import of `@/configure` under `frontend/src/stores`.
3. The store id `configDraft` and `draftStorageKey()` are unchanged: the moved
`configDraft.test.ts` (which asserts the key) and the e2e test "the draft
survives a reload and can be discarded" pass.
4. type-check, lint, unit tests, build and `just arch-lint` pass.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: mechanical file move)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: import reorganisation, no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: mechanical file move)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: frontend folder layout only; frontend/CLAUDE.md read)

**Research Doc:** N/A

**Existing Solutions:**

- Every `frontend/src/api/*.ts` module declares its own wire types next to its
calls, so the tree types go into `api/configure.ts` rather than a new file.
- Files inside `frontend/src/configure/` import their siblings with `./`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Move the type declarations and their encoding comment into `api/configure.ts`.
Change every importer to take the types from `@/api/configure`. `git mv` the
store and its test into `configure/` and rewrite `@/stores/configDraft` to
`@/configure/configDraft` (`./configDraft` inside `configure/`).

**Alternatives rejected:**

- A new `api/configureTree.ts`: no other api module splits its types out.
- Re-exporting the types from `configure/tree.ts`: it would keep the old
import path alive and hide where the types are owned.

**Files to modify:** `frontend/src/api/configure.ts`,
`frontend/src/configure/tree.ts`, the moved store and test, and the importers
under `frontend/src/{configure,components/configure,views/configure}`.

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: no new inputs; file moves only)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no new inputs)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: none touched)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no error handling changes)

**Input Sources & Validation:** N/A

**Security-Sensitive Operations:** N/A

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: no behaviour change; existing tests cover the store)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: no behaviour change)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1/AC2: grep checks.
2. AC3: `configure/__tests__/configDraft.test.ts`, `e2e/tests/configure.spec.ts`.
3. AC4: `npm run typecheck`, `npm run lint`, `npm run test:run`,
`npm run build`, `just arch-lint`.

**Edge Cases:** N/A

**Negative Tests:** N/A

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: no security surface)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A changed store id or storage key would orphan saved drafts. Mitigation: the
store body is moved unchanged; the key test and the reload e2e test run.
- A missed importer. Mitigation: vue-tsc and the build fail on it.

Effort: xs.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor, no docs impact)

**Documentation Impact:** N/A - internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design is the archplan handover, already reviewed by the architect in the archplan editor)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
