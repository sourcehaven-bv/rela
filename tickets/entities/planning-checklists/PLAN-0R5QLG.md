---
id: PLAN-0R5QLG
type: planning-checklist
title: 'Planning: Lint (incl. gosec) every backend build tag in CI'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: lint and gosec for the sqlite, postgres and memorybackend builds
in CI and `just lint`; clearing their existing findings. Out: other build tags
(windows, maildemo, mailmanual) and test jobs, which already run per backend.

**Acceptance Criteria:**
1. `golangci-lint run --build-tags <tag> ./...` reports 0 issues for default, sqlite, postgres and memorybackend.
2. The CI Lint job runs all four and fails on a finding.
3. `just lint` runs all four.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small CI change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: golangci-lint `--build-tags` is the standard mechanism)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** BUG-RNM0MU and AM-lint-every-build-tag (proposed)
describe the same gap for lint; `just tagged-tests` already vets the sqlite and
postgres builds.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Run golangci-lint once per tag, because the tags select
mutually exclusive backends. In CI use extra steps in the existing Lint job, not
a matrix, so the required check keeps its name. Fix the 20 postgres findings in
place.

**Files to modify:** .github/workflows/ci.yml, justfile, and the postgres-tagged
files lint reports (docscapture scratch backend,
pgqueue/pgstore/tenant/entitymanager/dataentry postgres tests).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** N/A: no new inputs.

**Security-Sensitive Operations:** None added. The change extends gosec to code
that ships in rela-server-postgres.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** 1: run all four lints locally. 2: the PR's Lint job. 3:
`just lint`.

**Edge Cases:** Files tagged `postgres || sqlite` are covered by either run.
Test fixes must keep behaviour: helpers that switch to t.Context() must not use
it after cancellation.

**Negative Tests:** A new finding in a tagged file fails the Lint job.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Lint job time grows (four runs); golangci-lint cache mitigates.
Postgres test fixes cannot run locally (no DB); CI runs them.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore with no docs impact)

**Documentation Impact:** N/A: internal CI change; the justfile comment
documents it.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: mechanical CI change; the code review covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
