---
id: PLAN-5ICWG
type: planning-checklist
title: 'Planning: Lint rule to flag pure-API test patterns in e2e specs'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

- IN: A spec-only eslint rule in `e2e/eslint.config.js` that bans `api.rawRequest(...)` calls inside `test(...)` callbacks.
- IN: Updated error message that names the alternative location (Go integration tests in `internal/dataentry/`) and points at `e2e/tests/AGENTS.md`.
- IN: A short rule-rationale section added to `e2e/tests/AGENTS.md`.
- OUT: Banning `api` use in specs entirely. The typed helpers (`createEntity`, `getEntity`, `listRelations`, etc.) are legitimate seed/verification tooling.
- OUT: AST-level "this test never touches `appPage`" detection. eslint's `no-restricted-syntax` is stateless per-node; counting sibling references in a callback would require a custom rule package, which is out of proportion for the size of the problem.
- OUT: Banning query-string paths in helper code (page objects / `tests/fixtures.ts`). The `api` fixture itself constructs query strings; that's fine.
- OUT: Removing existing tests. None match the anti-pattern today (verified: `grep -rn rawRequest e2e/tests/*.spec.ts` returns nothing).

**Acceptance Criteria:**

1. A new spec containing `api.rawRequest('GET', '...')` inside a `test(...)` body fails `npm run lint` in `e2e/`.
   - **Test scenario:** add a temporary `e2e/tests/_lint_canary.spec.ts` with the user's exact illustrative example, run `npm run lint`, expect non-zero exit and the new error message; remove the canary file before commit.
2. Existing specs lint clean.
   - **Test scenario:** `cd e2e && npm run lint` passes after the change with no source modifications to specs.
3. The rule does NOT flag `api.rawRequest` calls in `beforeAll` / `beforeEach` / `afterAll` / `afterEach` hooks.
   - **Test scenario:** add a temporary canary that calls `api.rawRequest` only in `test.beforeEach`, expect lint to pass; remove canary.
   - **Note on practicality:** eslint `no-restricted-syntax` selectors can match by ancestor function-call name. We restrict the ban to nodes whose nearest enclosing `CallExpression` is `test(...)` / `test.skip(...)` / `test.only(...)` / `test.fixme(...)`.
4. `e2e/tests/AGENTS.md` documents the rule, its rationale ("API-only assertions belong in Go integration tests"), and points at `internal/dataentry/handlers_api_test.go` as the right home.
5. The user's exact example (`api.rawRequest('GET', \`features/${SEED.features.exportData}/relations/blocks?direction=incoming\`)`inside`test(...)`) fires the rule.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **`no-restricted-syntax` (eslint built-in)** — already used in `e2e/eslint.config.js:37-60` to ban `locator`, `getByX`, `waitForTimeout`, `request.fetch`. Selector grammar: <https://eslint.org/docs/latest/extend/selectors>. Supports ancestor combinators (` ` / `>`) so we can match "CallExpression descended from a `test(...)` CallExpression."
- **eslint-plugin-playwright** — has rules like `no-raw-locators`, `expect-expect`, but no rule for "this test only hits API." Out of scope to add a dependency for one selector.
- **Codebase prior art:** the existing `request.fetch` ban (RR-3VPYE) is the precise template. Same shape, same `files: ['tests/**/*.spec.ts']` scope, same exemption for page objects (which don't apply here — `api` use in pages would be a separate code smell).

**Approach decision: AST selector vs. custom rule package.**

A custom eslint plugin could do exact "test body has `api.*` but no `appPage` /
page-object reference" detection. Rejected because:

- Maintenance overhead: separate package, separate publish/install, separate CI surface.
- The simpler proxy ("`api.rawRequest` inside `test(...)`") is precise enough — `rawRequest` is the un-typed escape hatch; if you're using it in a spec body, you're testing API shape. Typed helpers cover legitimate seed/verify uses.
- All existing legitimate `api` uses go through typed helpers. Banning the escape hatch in spec bodies puts the friction exactly where the smell lives.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Add a fourth entry to the existing `no-restricted-syntax` array in
`e2e/eslint.config.js` (the `files: ['tests/**/*.spec.ts']` block):

```js
{
  // Bans api.rawRequest(...) inside test(...) bodies. The typed helpers
  // (createEntity, getEntity, listRelations, etc.) cover legitimate seed
  // and verification use cases. rawRequest is the un-typed escape hatch —
  // if you need it in a spec body, you're testing HTTP API shape, which
  // belongs in Go integration tests under internal/dataentry/.
  selector:
    "CallExpression[callee.object.name='test'] CallExpression[callee.object.name='api'][callee.property.name='rawRequest'], " +
    "CallExpression[callee.name='test'] CallExpression[callee.object.name='api'][callee.property.name='rawRequest']",
  message:
    'Do not use api.rawRequest in spec test bodies — these are HTTP-shape assertions, ' +
    'which belong in Go integration tests (internal/dataentry/handlers_api_test.go). ' +
    'Use the typed api helpers (createEntity / getEntity / listRelations) for seeding ' +
    'and verification of UI tests. See e2e/tests/AGENTS.md.',
},
```

Notes:

- The two-form selector covers both `test('name', cb)` (where `callee.name='test'`) and `test.describe(...)`, `test.skip(...)` etc. (where `callee.object.name='test'`). Setup hooks like `test.beforeEach(...)` also fall under `callee.object.name='test'`, so they ARE matched by the bare descendant selector. To exempt them, we tighten: only descend from `CallExpression` whose `callee` is `test`, `test.only`, `test.skip`, or `test.fixme` — i.e., a property name in {only, skip, fixme} or a bare `test` identifier. We exclude `beforeAll`, `beforeEach`, `afterAll`, `afterEach`.
- Final selector grammar (split for readability):
  - `CallExpression[callee.name='test'] CallExpression[callee.object.name='api'][callee.property.name='rawRequest']`
  - `CallExpression[callee.object.name='test'][callee.property.name=/^(only|skip|fixme)$/] CallExpression[callee.object.name='api'][callee.property.name='rawRequest']`

Then update `e2e/tests/AGENTS.md` with a new subsection under "Page Object
Pattern (enforced)" or as a sibling: "API-only tests belong in Go," explaining
the boundary and pointing to `internal/dataentry/handlers_api_test.go`.

**Files to modify:**

- `e2e/eslint.config.js` — add the selector pair.
- `e2e/tests/AGENTS.md` — document the rule and rationale.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- N/A — this is a static lint rule. It doesn't run at runtime. The only "input" is dev-authored test source code, parsed by eslint.

**Security-Sensitive Operations:**

- None. No file I/O, no network, no crypto.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1 (rule fires in spec test body): create a throwaway `e2e/tests/_canary_positive.spec.ts` containing the user's example inside a `test(...)`, run `npm run lint`, confirm error fires with the expected message; delete the canary.
- AC2 (existing specs unaffected): run `npm run lint` on the unmodified spec tree after the rule lands; expect zero new errors.
- AC3 (setup hooks exempt): canary spec calling `api.rawRequest` only in `test.beforeEach`, expect lint to pass; delete the canary.
- AC4 (docs updated): visual check `e2e/tests/AGENTS.md` mentions the new rule, rationale, and Go integration test pointer.
- AC5 (user's exact example): direct paste of `api.rawRequest('GET', \`features/${SEED.features.exportData}/relations/blocks?direction=incoming\`)` triggers the rule.

**Edge Cases:**

- `api.rawRequest` aliased to a local name: e.g. `const r = api.rawRequest; r('GET', ...)`. **Not detected.** Acceptable; this is contortion-to-bypass, and reviewer will catch it. Documented as known limitation.
- `await api['rawRequest'](...)` (computed property). **Not detected** by the dotted selector. Same call: a deliberate bypass; not a realistic dev pattern.
- Setup hooks (`beforeEach` / `beforeAll` / `afterEach` / `afterAll`): exempt by selector design.
- Page objects calling `api.rawRequest`: the rule is scoped to `tests/**/*.spec.ts`, so page objects under `pages/` are unaffected (matches existing exemption pattern).

**Negative Tests:**

- Spec uses `api.createEntity` / `api.listRelations` / `api.getEntity` only → must NOT fire.
- Spec uses `api.rawRequest` in `test.beforeEach` → must NOT fire.
- Spec uses `api.rawRequest` directly inside `test(...)` body → MUST fire.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Risk:** Selector syntax bugs that either over-match (catches setup hooks) or under-match (misses test bodies). **Mitigation:** validate against canary specs before finalizing. Run on the existing tree to confirm zero false positives.
- **Risk:** `eslint-plugin-typescript` may not honor descendant combinators in nested `tseslint.config()` blocks. **Mitigation:** the existing `request.fetch` rule uses descendant selectors successfully, so the grammar is known-supported.
- **Risk:** Over-blocking — devs hit the rule for genuinely needed escape hatches (e.g., a CSRF-canary test). **Mitigation:** error message tells them to either use a typed helper or write the test as a Go integration test. There is one legitimate exception (origin-security canary in AGENTS.md:43-49) — it's already handled by writing the test in Go, not e2e, per existing guidance.

**Effort:** s (small) — one config change + a docs paragraph.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] CLAUDE.md (no — internal e2e tooling, not project-wide)
- [x] README.md (no)
- [x] API docs (no)
- [x] ~~User guide / reference docs~~ (N/A: dev-facing tooling, not user-facing)
- [x] **AGENTS.md (`e2e/tests/AGENTS.md`)** — yes, document the new rule and the boundary between e2e and Go integration tests.

This is dev tooling, so the "docs" are restricted to `e2e/tests/AGENTS.md`. A
separate `docs-checklist` is not warranted.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: ~25-line eslint config change in a single file, design fully captured in this checklist's Approach section. A separate design-review pass would just re-state what's here. The cranky-code-reviewer agent ran during the review phase and surfaced one significant finding — RR-C7AI9 — which has been addressed.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-C7AI9 (significant), RR-UTD3R (minor), RR-1O10G (minor), RR-GYIPB (minor) — all addressed.
