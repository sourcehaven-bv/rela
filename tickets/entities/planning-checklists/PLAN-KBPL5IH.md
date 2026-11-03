---
id: PLAN-KBPL5IH
type: planning-checklist
title: 'Planning: Bump vitest to 5.0.5 for GHSA-p458-7748-mcp3 (GH #1989)'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In scope: GHSA-p458-7748-mcp3 from GitHub #1989 (`@vitest/browser`
and `@vitest/ui` 5.0.3), and the three newer osv-scanner findings on the same
lockfile: GHSA-mrwp-f3q8-3g9p (follow-redirects), GHSA-3g3m-2fjr-6pw4 (glob) and
GHSA-g3hv-75wg-m249 (minimatch). Out of scope: the nested `@vitest/expect` 3.2.4
tree, which the advisory does not cover, and the vitest 5.1 beta.

**Acceptance Criteria:**
1. osv-scanner on `frontend/package-lock.json` reports no findings.
2. `npm ls vitest @vitest/ui @vitest/browser` shows 5.0.5 only.
3. Frontend typecheck, lint, unit tests and build pass; component library checks and tests pass.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: dependency bump)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: patch release of the existing dependency)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows TKT-L262IN)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A.

**Existing Solutions:** TKT-L262IN handled the previous osv-scanner issue
(#1924). Vitest 5.0.5 is the latest stable 5.0 release and contains the fix.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Raise the `^5.0.3` ranges to `^5.0.5` and regenerate the
lockfile entries of the vitest 5 packages. The packages declare exact peer
versions of each other, so a plain `npm install` refuses to move one alone; the
5.0.3 lock entries are dropped and re-resolved together.

follow-redirects 1.16.2 is within axios's `^1.16.0`, so `npm update` moves it.
glob and minimatch are fixed only in new majors (14.0.2 and 11.0.1), and no
release of eslint, `@eslint/config-array`, typescript-eslint, editorconfig or
js-beautify accepts them. They get npm `overrides` (`glob ^14.0.2`, `minimatch
^11.0.1`). The consumers only construct `Minimatch` with `{ dot: true }` and
call `globSync`; none use the APIs that minimatch 11 and glob 14 remove
(`makeRe`, `regexp`, `AST`, `debug`, `matchOne`).

Alternative rejected for vitest: an npm `overrides` entry. A direct range bump
is simpler and needs no later cleanup.

**Files to modify:** `frontend/package.json`,
`frontend/packages/rela-components/package.json`, `frontend/package-lock.json`.

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: dev-only test tooling)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no rela code changes)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no rela code changes)

**Input Sources & Validation:** N/A.

**Security-Sensitive Operations:** None in rela. The vulnerable packages run
only in local and CI test runs, not in shipped builds.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: dependency patch bump)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: dependency patch bump)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Run osv-scanner on the lockfile; run `npm ls`; run the
frontend CI steps (typecheck, lint, `RELA_FULL_CORPUS=1 npm run test:run`,
build) and the component library steps (`vue-tsc -b`, `npm run check`, `npm
test` in a real browser).

**Edge Cases:** N/A.

**Negative Tests:** N/A.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A patch release could change test behaviour. Mitigation: the full
frontend and component library suites run. Effort: xs.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: no user-facing change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore)

**Documentation Impact:** N/A.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: xs dependency bump with no design surface)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** None.
