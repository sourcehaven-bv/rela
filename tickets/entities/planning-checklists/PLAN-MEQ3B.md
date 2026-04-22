---
id: PLAN-MEQ3B
type: planning-checklist
title: 'Planning: Consolidate frontend/e2e into /e2e with fixes and CI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope (in):**

- Consolidate all Playwright e2e coverage into `/e2e/`.
- Delete `/frontend/e2e/` and its Playwright tooling in `frontend/package.json`.
- Switch `/e2e/` fixture from a module-level shared server to per-test backend on unique ports → `workers: parallel`.
- Enforce Page Object Pattern via **eslint `no-restricted-syntax`** applied to `e2e/tests/**/*.spec.ts`.
- Fix all ported tests that failed under `/frontend/e2e/`.
- Remove overlapping coverage.
- Wire the CI `e2e` job to keep gating `build`; add lint step to the job.

**Acceptance Criteria:**

1. `/frontend/e2e/` deleted; `frontend/package.json` no longer has Playwright devDeps or `test:e2e` scripts.
2. `just e2e` runs the full consolidated suite with every test passing locally.
3. CI `e2e` job triggers on PR to `main`/`develop` and gates the `build` job.
4. eslint fails on any spec using raw selectors, `waitForTimeout`, or `request.fetch`.
5. Each test starts its own backend on a unique port.
6. Unique coverage from `/frontend/e2e/` present in `/e2e/`.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

Reference: `/frontend/e2e/fixtures.ts` per-test backend pattern; eslint core
`no-restricted-syntax` is sufficient (no custom plugin needed).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

See commit history for phased execution: per-test backend, eslint config, port
unique specs, delete /frontend/e2e/, CI wiring, AGENTS.md.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

Server spawned with `-allowed-origin http://localhost:<port>`; origin-security
spec exercises the allowlist directly for GET/POST/PATCH/DELETE.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Effort:** m (1-2 days).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation (N/A — internal refactor)

**Documentation Impact:**

- [x] ~~User guide / reference docs~~ (N/A: internal test-infra refactor)
- [x] ~~CLI help~~ (N/A)
- [x] ~~CLAUDE.md (root)~~ (N/A: no change)
- [x] ~~README.md~~ (N/A: no change)
- [x] ~~API docs~~ (N/A)
- [x] `frontend/CLAUDE.md` — drop E2E Tests section
- [x] `e2e/tests/AGENTS.md` — new, with page-object guidance

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 19 responses (RR-B8GJT, RR-17XTS, RR-K6DJL,
RR-BZUH5, RR-3VPYE, RR-3DJ2C, RR-F3IA3, RR-LWG6W, RR-J9BIT, RR-GX4BK, RR-0RDB4,
RR-26RE6, RR-SG0LP, RR-2TFDO, RR-65GPQ, RR-9WOSL, RR-MS1FM, RR-VKXY2, RR-V63DT)
— criticals and significants all addressed; minor/nit split addressed/deferred
with reasons.
