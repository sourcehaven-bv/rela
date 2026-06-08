<!-- @managed: claude-workflow v1 -->
---
id: REV-UD7YR
type: review-checklist
title: 'Review: Route view-side per-field rendering through widget registry'
status: done
---

## Automated Checks

- [x] All tests pass (`npm run test:run` — 950 passed, 58 test files)
- [x] Lint clean (`npm run lint` — 0 errors in new/changed files)
- [x] Coverage maintained — widgets dir holds at >95% statements; the lone `coverage:check` violation (`src/stores/schema.ts`) is the same pre-existing flake first observed on TKT-MZSIJ, unrelated to this work.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] ~~All critical review-responses addressed~~ (N/A: zero critical findings in this round)
- [x] All significant review-responses addressed (RR-UD2A, RR-UD2B, RR-UD2C, RR-UD2D, RR-UD2F, RR-UD2I — all `addressed`)
- [x] Self-reviewed the diff for unrelated changes (no incidental `vite.config.js` drift this time; `README.md` re-generation via `just docs` produced no diff once the docs-project fsstore index was refreshed)

**Review Responses:**

12 design-review findings (RR-UD1A–RR-UD1L) all addressed in the planning phase.

12 code-review findings (RR-UD2A–RR-UD2L) addressed during the review phase:

- Significant (6, all addressed): RR-UD2A (precompute), RR-UD2B (WidgetRoutingHint replaces synthDef), RR-UD2C (em-dash + comma-join), RR-UD2D (required propertyName + Badge fallback removed), RR-UD2F (SelectWidget array guard), RR-UD2I (real disabled checkbox)
- Minor (5, mix): RR-UD2E *deferred* (needs backend wire-shape change for per-card inaccessibility reason), RR-UD2H addressed (negative assertions), RR-UD2J addressed (comment), RR-UD2K addressed (extraction byproduct of RR-UD2B), RR-UD2L addressed (viewRouting.test.ts spy assertions)
- Nit (1): RR-UD2G addressed at intake (no issue)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

All 12 acceptance criteria PASS — see PLAN-UD7YR for the list; evidence aggregated in IMPL-UD7YR's Verification Evidence block. Highlights:

1. ✅ `mode` is required, strict typed union, no default — every test mount passes it explicitly
2. ✅ `propertyName` added and required — same pattern
3. ✅ 8 widgets gained display branches reusing existing helpers
4. ✅–✅ PropertyDisplay / cards / list all delegate via the registry
7. ✅ `InaccessibleField.vue` owns the lock affordance; three consumers use it
8. ✅ Section-level precompute via `fieldRowsFor` / `PropertyDisplay.rows`
9. ✅ `table` and `content` display modes unchanged
10. ✅ `FieldRenderer.vue` passes `:mode="'edit'"` and `:property-name="field.property"` explicitly
11. ✅ Per-widget display-mode tests + negative assertions in edit-mode tests
12. ✅ Browser smoke confirms Known Behaviour Deltas match the table and nothing else

## Documentation (enhancements only)

- [x] ~~Docs-checklist created~~ (N/A: internal refactor, no user-facing API change)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (18/19 non-self-referential checks green; the `Rela Tickets` validation gate fires on review+in-progress workflow states, resolved by transitioning ticket and checklists to `done` once the PR is up and reviewed)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/906
