---
id: TKT-4Q2VI
type: ticket
title: Consolidate frontend/e2e into /e2e with fixes and CI
kind: refactor
priority: medium
effort: m
tags:
    - tech-debt
status: review
---

## Problem

Two Playwright E2E suites exist:

- `/e2e/` — 88 tests, all passing, wired into CI (`e2e` job, required by `build` gate).
- `/frontend/e2e/` — 152 tests, 26 failing, **not** in CI. Decaying.

Both boot the real `rela-server` Go binary. The only real difference is that
`/frontend/e2e/` serves the SPA from Vite dev server (:5173) and routes `/api/*`
via `page.route()` to a per-test backend on a random port, while `/e2e/` has the
Go binary serve its embedded SPA directly. The Vite-vs-built-bundle delta is
small — the `frontend` CI job already builds and type-checks the SPA — so
exercising the dev server adds complexity (proxy, page.route, Origin header
workarounds) without meaningful coverage of the shipped artifact.

### Current failures in `/frontend/e2e/`

- API tests using raw `request.get/put` without an Origin header → 403 origin_missing (Settings API, Analyze API, Conflicts API, Search API, entity delete)
- Graph explorer tests (feature was removed — commit b66f598)
- Stale selectors (e.g. "shows all sections" expects 4 settings cards, now 5 after palette card added)
- Miscellaneous decay: dark mode, dashboard, checkbox toggling, relation-cards batch save

## Goal

Single E2E suite at `/e2e/`, running against the built `rela-server` binary.
Unique coverage from `/frontend/e2e/` ported over. All tests pass. Wired into CI
on PRs. Strict Page Object Pattern (no raw CSS selectors, no `page.locator(...)`
calls, no `waitForTimeout` in specs).

## Scope

- Port genuinely-unique coverage from `/frontend/e2e/` into `/e2e/` (dashboard critical-issues, relation-cards batch save, palette settings, checkbox toggling, dark mode, analyze/conflicts/search API coverage).
- Drop `/frontend/e2e/` entirely. Remove `test:e2e` scripts and `@playwright/test` from `frontend/package.json`.
- Enforce Page Object Pattern: tests only call page-object methods. No `page.locator`, no CSS selectors in specs.
- Fix all ported tests so they pass.
- Use per-test backend with unique ports (already how `/frontend/e2e/` works; port to `/e2e/`).
- Ensure `e2e` CI job still runs on PRs and gates `build`.
- Remove overlapping tests (same UI behavior covered in both suites — keep one).

## Out of scope

- New test coverage for untested areas.
- Migrating unit tests.
- Changes to `rela-server` or SPA source code (unless a real product bug surfaces during porting).

## Acceptance criteria

1. `/frontend/e2e/` directory is deleted; `frontend/package.json` no longer has Playwright dependencies or `test:e2e` scripts.
2. `/e2e/` contains all unique coverage from both suites, with no duplicated tests.
3. `just e2e` runs all consolidated tests; all pass locally.
4. CI `e2e` job runs on PRs to `main`/`develop`, is required by `build`, and passes.
5. No spec file contains raw CSS selectors or `page.locator(...)` calls — all UI interaction goes through page objects.
6. Each test gets its own backend instance on a unique port (parallelizable).
