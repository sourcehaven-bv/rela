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
- Switch the consolidated suite to use `prototypes/data-entry/project` as the test project.
- Switch `/e2e/` fixture from a module-level shared server to per-test backend on unique ports → `workers: parallel`.
- Enforce Page Object Pattern via **eslint `no-restricted-syntax`** applied to `e2e/tests/**/*.spec.ts` (page objects themselves remain free to use selectors). Banned in specs: `*.locator(...)`, `*.getByRole/Text/TestId/Label/Placeholder/Title/AltText(...)`, `*.waitForTimeout(...)`.
- Fix all ported tests that failed under `/frontend/e2e/`.
- Remove overlapping coverage.
- Wire the CI `e2e` job to keep gating `build`; add lint step to the job.

**Scope (out):**

- New behavioral coverage beyond what already exists.
- Refactoring spec files unrelated to selector/pattern compliance.
- Changes to `rela-server` or SPA source (only if a real product bug surfaces).
- Vite dev server tests. `frontend` CI job still runs `npm run build`.

**Acceptance Criteria (with test scenarios):**

1. `/frontend/e2e/` directory and `frontend/package.json` Playwright devDep/scripts are deleted.
   - Verify: `ls frontend/e2e` → no such path; `grep -E "playwright|test:e2e" frontend/package.json` → no matches.
2. `just e2e` runs the full consolidated suite with every test passing locally.
   - Verify: `just e2e` exits 0; report shows N passed, 0 failed, 0 flaky.
3. CI `e2e` job triggers on PR to `main`/`develop` and gates the `build` job.
   - Verify: `.github/workflows/ci.yml` still has `e2e` in `build.needs`.
4. eslint fails on any spec using raw selectors or `waitForTimeout`.
   - Verify: add an intentionally-violating test as a smoke check during development; confirm eslint flags it; remove the violation; eslint passes. In CI: `e2e` job runs `npm run lint` before `npx playwright test`.
5. Each test starts its own backend on a unique port.
   - Verify: `workers: process.env.CI ? 2 : undefined`; local run with 4 workers passes.
6. Unique coverage from `/frontend/e2e/` is present in `/e2e/`: dashboard Critical-Issues, relation-cards batch save + unsaved-badge, dark-mode, checkbox toggling, analyze/conflicts page + API, richer entity-detail.
   - Verify: each listed test has a named equivalent in `/e2e/tests/` and passes.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- Both suites already use Playwright + Page Object pattern. No library swap.
- `/frontend/e2e/fixtures.ts` demonstrates per-test unique-port backend, temp-project copy, worker-scoped `serverBinary`/`relaCLI`. That's the target.
- `/frontend/e2e/page-objects/` is richer than `/e2e/pages/`; port the class-based factory pattern.
- Prior work: TKT-31JX ("Vue migration parity fixes and E2E test coverage"). Related review-responses: RR-EEK5, RR-C0FYK, RR-UHIF6.
- For lint enforcement: eslint core `no-restricted-syntax` rule uses esquery selectors against the AST. No custom plugin required; `typescript-eslint` parser handles TS syntax.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Phase 1 — switch `/e2e/` fixture to per-test backend.

- Rewrite `e2e/tests/fixtures.ts`: copy `prototypes/data-entry/project`, set up `.rela/bin/rela`, per-test `spawn`, `findFreePort`, `waitForServer` with Origin-header probe. Drop module-level globals.
- `browser.newContext({ extraHTTPHeaders: { Origin: serverUrl } })` so `request.*` calls pass Origin.
- Worker-scoped `serverBinary` fixture.
- Remove `workers: 1`.
- Rename entity types in existing 7 specs: `feature`/`bug`/`task` → `ticket`/`category`/`label`. ~100-line mechanical diff.

Phase 2 — port unique frontend/e2e coverage.

| From `/frontend/e2e/` | Into `/e2e/tests/` |
|---|---|
| `analyze.spec.ts` | new `analyze.spec.ts` |
| `conflicts.spec.ts` | new `conflicts.spec.ts` |
| `dashboard.spec.ts` | new `dashboard.spec.ts` |
| `entity-detail.spec.ts` | new `entity-detail.spec.ts` |
| `forms.spec.ts` | new `forms.spec.ts` |
| `misc-features.spec.ts` (Dark Mode, Keyboard, Markdown, Checkbox Toggling) | split into `theming.spec.ts`, `keyboard.spec.ts`, `markdown.spec.ts`, `checkboxes.spec.ts` |
| `relation-cards.spec.ts` | new `relation-cards.spec.ts` |
| `search.spec.ts` (API with type filter) | merge into existing `search.spec.ts` |
| `settings.spec.ts` (API, overrides) | merge into existing `settings.spec.ts` |
| `document-live-update.spec.ts` | skipped — all source tests are `test.skip` |

Overlapping (drop the frontend version): navigation, CRUD read/update/delete,
kanban display, list display, settings display — keep `/e2e/` version.

Page-object additions in `/e2e/pages/`:

- `DashboardPage` (Critical-Issues card, `navigateToSidebarItem`).
- `AnalyzePage`, `ConflictsPage`.
- Relation-cards helpers (on `EntityPage` or a dedicated `RelationCardsPage`): `addRelation(type, target, props)`, `editCardProperty(...)`, `save()`, `unsavedBadgeCount()`.
- `FormPage`: `getTemplatePills()`, `applyTemplate()`, `typeInBodyEditor()`.
- `SettingsPage`: API wrappers so specs don't touch `request` directly.
- `api` fixture (from frontend/e2e) — injects Origin on all HTTP requests.

Phase 3 — fix failures as tests are ported.

- Origin-missing (13 tests): go through `api` fixture.
- Stale card-count assertion: rewrite as "each named card visible" via page object.
- Graph explorer (2 tests): delete.
- Palette card: assert via page object or drop.
- Relation-cards batch save: if unfixable in test, open a BUG, skip with TODO linking it, surface to user.

Phase 4 — eslint + CI + cleanup.

- Add `e2e/eslint.config.js` with:
  - `@typescript-eslint/parser`, `@typescript-eslint/eslint-plugin` (devDep additions to `e2e/package.json`).
  - Override block scoped to `tests/**/*.spec.ts`:
    ```
    'no-restricted-syntax': ['error',
      { selector: "CallExpression[callee.property.name=/^(locator|getByRole|getByTestId|getByText|getByLabel|getByPlaceholder|getByTitle|getByAltText)$/]",
        message: 'Use a page object method instead of calling selectors directly in specs.' },
      { selector: "CallExpression[callee.property.name='waitForTimeout']",
        message: 'Use waitFor/expect-retry instead of fixed timeouts.' }
    ]
    ```
  - `npm run lint` script in `e2e/package.json`.
- CI `e2e` job: insert a `Run lint` step after `Install e2e dependencies`, before Playwright install.
- Delete `frontend/e2e/`, `frontend/playwright.config.ts`. Remove `@playwright/test`, `test:e2e`, `test:e2e:ui` from `frontend/package.json`. Update `frontend/CLAUDE.md` (drop E2E section).
- New `e2e/tests/AGENTS.md` with page-object guidance (substance from `frontend/e2e/AGENTS.md`).

**Files to modify:**

- `e2e/tests/fixtures.ts` — full rewrite.
- `e2e/playwright.config.ts` — drop `workers: 1`, bump timeout to 30s.
- `e2e/pages/*.ts` — extend + add new page objects.
- `e2e/tests/*.spec.ts` — add new, update existing.
- `e2e/package.json` — add eslint + typescript-eslint devDeps + `lint` script; bump `@playwright/test` to 1.58.2.
- `e2e/eslint.config.js` — new.
- `e2e/tests/AGENTS.md` — new.
- `frontend/package.json` — drop Playwright devDep + scripts.
- `frontend/playwright.config.ts` — delete.
- `frontend/e2e/` — delete entire directory.
- `frontend/CLAUDE.md` — drop E2E Tests section.
- `.github/workflows/ci.yml` — add `npm run lint` step to `e2e` job.
- `justfile` — no change expected.
- `CLAUDE.md` — no change.

**Alternatives considered:**

- Keep both suites. Rejected: rot proved it.
- Retire `/e2e/`, keep `/frontend/e2e/`. Rejected: tests dev-mode SPA, not shipped artifact.
- Delete `/frontend/e2e/` without porting unique coverage. Rejected: loses real coverage.
- CI `grep` for raw selectors instead of eslint. Rejected: false positives on strings in comments, fragile, no IDE integration. eslint is standard Node tooling.
- Custom eslint plugin. Rejected: overkill. Core `no-restricted-syntax` is sufficient.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Spawns `bin/rela-server` on random localhost port. Origin set to same baseUrl.
- Temp project under `os.tmpdir()` with `rela-e2e-<pid>-<rand>` prefix. YAML/Markdown under our control. Cleaned in teardown.

**Security-Sensitive Operations:**

- Per-test fs writes to `os.tmpdir()`: bounded, unique prefix, cleaned.
- No auth/crypto in tests.
- Server launched with `-allowed-origin http://localhost:<port>`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

| AC | How it is tested |
|---|---|
| 1. frontend/e2e deleted | `just e2e` still passes; `grep -r "frontend/e2e" .github/ justfile frontend/ CLAUDE.md` → no hits. |
| 2. just e2e passes | Full Playwright run. |
| 3. CI gates build | Re-push branch; GitHub Actions `e2e` job required. |
| 4. eslint flags selectors in specs | Add a `page.locator(...)` call to any spec temporarily during dev; confirm `npm run lint` fails; remove; passes. |
| 5. Parallel workers | Remove `workers: 1`; local run with `--workers=4` passes. |
| 6. Unique coverage ported | Each named test present and passing. |

**Edge Cases:**

- Port collision: `findFreePort` returns bound port; retry on `EADDRINUSE`.
- Slow server boot: `waitForServer` 30s with stdout/stderr dump on fail.
- Cleanup failure: try/catch around `fs.rmSync`, log, don't fail test.
- Browser/server process leak: SIGTERM in teardown, `process.on('exit')` fallback.
- Parallel flakiness: mark individual `describe` as `serial`, never global.

**Negative Tests:**

- Backend security not regressed (unit tests in `internal/dataentry` cover missing-Origin → 403).
- Invalid entity type → `api.createEntity` throws.
- Entity not found: keep "404 for invalid entity" test.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**

| Risk | Impact | Likelihood | Mitigation |
|---|---|---|---|
| Relation-cards batch save is actually broken in product | Medium | Medium | File BUG, skip with TODO, surface to user. |
| Per-test backend spin-up blows out CI time | Medium | Low | ~200-300ms per test; parallel with 2 CI workers offsets. |
| Port exhaustion on CI | Low | Low | Ephemeral range; ~150 tests with teardown → fine. |
| Prototype project lacks needed fixtures | Medium | Medium | `/frontend/e2e/` already uses it; baseline known good. |
| Prototype drift breaks e2e | Low | Low | Document in `e2e/tests/AGENTS.md` as test dependency. |
| Entity-type rename touches every spec | Medium | High | Phase 1 is mechanical single commit. |
| eslint rule catches too much / too little | Low | Medium | `no-restricted-syntax` selectors are precise; test by introducing a known-bad line and confirming the error message. |

**Effort:** m (1-2 days).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [ ] User guide / reference docs — N/A
- [ ] CLI help — N/A
- [ ] CLAUDE.md (root) — no change
- [ ] README.md — no change
- [ ] API docs — N/A
- [x] `frontend/CLAUDE.md` — drop E2E Tests section
- [x] `e2e/tests/AGENTS.md` — new, with page-object guidance

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** TBD.
