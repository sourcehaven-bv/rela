---
id: PLAN-WFYK320
type: planning-checklist
title: 'Planning: e2e: buildIfMissing passes -o after the package path'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** `buildIfMissing` in `e2e/tests/fixtures.ts` and its two callers. Out
of scope: CI build steps, which already build the binaries first.

**Acceptance Criteria:**
1. With the binary absent, `buildIfMissing` builds `rela-server` (default build).
2. With the binary absent, `buildIfMissing` builds the postgres server with `-tags postgres`.
3. The signature separates flags from the package so `-o` cannot land after it.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: xs chore)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: argument-order bug)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: argument-order bug)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: argument-order bug)

**Research Doc:** N/A

**Existing Solutions:** Only `buildIfMissing` invokes `go build` in `e2e/`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Change the signature to
`buildIfMissing(binaryPath, pkg, flags = [])` and build argv as
`["build", ...flags, "-o", binaryPath, pkg]`. The ticket's suggested
`["build", "-o", binaryPath, ...buildArgs]` was rejected because it keeps flags
and the package in one rest parameter, so a caller can still put a flag after
the package.

**Files to modify:** `e2e/tests/fixtures.ts`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: arguments are hard-coded test constants)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Hard-coded constants in the fixture file only.

**Security-Sensitive Operations:** Process execution stays `execFileSync` with
an argv array and no shell.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: no new input handling; the CI guard is unchanged)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Extract the function body from `fixtures.ts`, run it with
node against absent output paths for both callers, and check the postgres
binary's build info shows `-tags=postgres`. Run `npm run typecheck` and eslint.

**Edge Cases:** Default `flags` of `[]` for the plain build.

**Negative Tests:** N/A.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** None beyond local e2e runs; CI does not reach this path. Effort: xs.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: test fixture only)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, not enhancement)

**Documentation Impact:** N/A - internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: xs argument-order fix; self-review is enough)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
