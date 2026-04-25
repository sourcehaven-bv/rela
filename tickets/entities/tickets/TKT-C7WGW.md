---
id: TKT-C7WGW
type: ticket
title: Lint rule to flag pure-API test patterns in e2e specs
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

E2E specs occasionally drift into testing REST API shape directly, e.g.:

```ts
const resp = await api.rawRequest(
  'GET',
  `features/${SEED.features.exportData}/relations/blocks?direction=incoming`,
);
```

This is the wrong layer:

- E2E tests boot a full browser + Go binary per test → seconds of overhead per assertion.
- The test isn't exercising the SPA at all; it's exercising the HTTP handler.
- Equivalent coverage in Go integration tests (`internal/dataentry/handlers_api_test.go` style) runs in milliseconds, with proper table-driven coverage of edge cases.
- The Playwright suite is where browser/UI behavior is verified — pure API assertions there are out of scope and slow the suite down.

The existing eslint config in `e2e/eslint.config.js` already enforces the Page
Object Pattern (no `locator`, no `request.fetch`). It does **not** catch specs
that legitimately use the `api` fixture but never touch the browser — i.e.
API-only specs.

## Goal

A spec-only eslint rule that flags API-only test bodies, with a clear error
message pointing devs to the Go integration test layer.

## Scope

- New `no-restricted-syntax` (or custom) rule in `e2e/eslint.config.js`.
- Detect `test(...)` callbacks where `api.rawRequest(...)` (or a freshly-introduced API verb) is used without any page-object interaction in the same callback.
- Error message names the alternative: `internal/dataentry/handlers_api_test.go` for handler tests, plus a pointer to `e2e/tests/AGENTS.md`.
- Update `e2e/tests/AGENTS.md` to document the new rule and the API-vs-UI test split.

## Out of scope

- Removing or rewriting any existing tests (none currently match this anti-pattern in the tree as of this ticket).
- Banning `api` fixture use in specs entirely — `api.createEntity` for seeding and `api.listRelations`/`api.getEntity` for post-action verification are legitimate and must keep working.
- Banning query-string paths outright in helper code (page objects / `tests/fixtures.ts`).

## Acceptance criteria

1. Adding a spec that uses only `api.*` calls (no `appPage`, no page-object usage) fails `npm run lint` in `e2e/` with a message that points to Go integration tests.
2. Existing specs continue to lint clean (verified: `relation-cards.spec.ts`, `markdown-editor.spec.ts`, `forms.spec.ts`, `checkboxes.spec.ts` all use `api.*` alongside page-object UI assertions).
3. The rule does NOT flag setup-only `api` calls in `beforeAll` / `beforeEach` / `afterEach` / `afterAll` hooks.
4. `e2e/tests/AGENTS.md` documents the rule, its rationale, and the boundary between e2e and Go integration tests.
5. The rule fires on the user's exact illustrative example (raw `api.rawRequest('GET', '...?direction=incoming')` inside a `test(...)` body with no UI assertions).
