# E2E Test Guidelines

These tests run Playwright against the built `rela-server` binary. Each test
gets its own backend on a unique port, its own temp project under
`$TMPDIR/rela-e2e-*`, and its own browser context. The inline project schema
and seed data live in `fixtures.ts`.

## Page Object Pattern (enforced)

Specs in `tests/**/*.spec.ts` **must not** call these Playwright APIs directly:

- `*.locator(...)`
- `*.getByRole/Text/TestId/Label/Placeholder/Title/AltText(...)`
- `*.waitForTimeout(...)`
- `request.fetch(...)`

These live in page objects under `../pages/` or the `api` fixture. Violations
are a compile-level eslint error (`no-restricted-syntax`, configured in
`e2e/eslint.config.js`). Why:

- **Stability.** Selectors change when the SPA changes; page objects isolate
  the churn to a single file per view.
- **Readability.** A spec reads as a user story; `.locator('.thing-v3 > span')`
  buries the intent.
- **CSRF safety.** The Origin allowlist is checked on every request.
  `api.rawRequest(...)` always sets `Origin`; bare `request.fetch(...)` does
  not, and silently 403s.

If you need a selector that doesn't exist in a page object yet, add a method
to the page object rather than inlining the selector in a spec. See
`pages/base.page.ts` for the base class.

## Fixtures

| Fixture | What you get |
|---|---|
| `appPage` | A Page attached to a fresh backend, pre-navigated to `/`. |
| `serverUrl` | The backend's base URL, e.g. `http://localhost:54321`. |
| `api` | HTTP helpers that auto-inject the matching `Origin` header. |
| `testProject` | Absolute path to the temp project directory. |
| `serverBinary` (worker-scoped) | Path to `bin/rela-server`. CI pre-builds it; locally the fixture builds on demand, serialised via a lockfile. |

## Security canary

`tests/origin-security.spec.ts` verifies the Origin allowlist rejects
missing/mismatched origins and admits the matching one. It uses
`playwrightRequest.newContext()` to bypass the `api` fixture — do not remove
or weaken this spec; it is the only coverage of the middleware itself.

## Test project

The inline metamodel (`METAMODEL_YAML`) defines three entity types:
`feature`, `bug`, `task`. Relations: `blocks` (with properties), `tagged`,
`implements`, `fixes`. No automations or validation rules — if a test needs
those, extend the YAML in `fixtures.ts`.

Do **not** point the fixture at `tickets/` (the real design/issue tracker
that rela dogfoods on itself). That's load-bearing production data for the
project, not a test fixture.

## Writing a new test

1. Figure out which view the test exercises (`/analyze`, `/dashboard`, etc.).
2. Check if there's a matching page object in `pages/`. If yes, extend it.
   If no, create one modelled on e.g. `pages/analyze.page.ts`.
3. Seed whatever entities you need via `api.createEntity(...)`. Clean up in
   `test.afterEach` (use `.catch(() => {})` on cleanup deletes).
4. Write the spec using only page-object methods for UI interaction.
5. Run `npm test -- <file>` — if it flakes under parallel load, prefer
   `expect.poll`/`toBeVisible` retries over `waitForTimeout` (and eslint will
   reject that anyway).
