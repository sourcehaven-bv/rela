---
id: IMPL-BJSQWA
type: implementation-checklist
title: 'Implementation: Relation filter_controls render as target selector (select → typeahead), not free text'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`EntityTargetSelect.test.ts` 15 tests, `FilterBar.test.ts` 9 tests)
- [x] Integration tests written (e2e `list.spec.ts` — relation filter narrows the real server's list end-to-end)
- [x] Happy path implemented (select ≤10, typeahead >10, value = bare `_title`)
- [x] Edge cases from planning handled (0/boundary counts, dup titles, missing `_title`, cancelled fetch, multi-source-type merge)
- [x] Error handling in place (`isCancelledFetch`-suppressed; other errors `console.error`'d, widget degrades to empty)

## Test Quality

- [x] Using fixture builders (`persoon()`, `manyCandidates()`, `seedRelationType()`, `stubCandidates()`)
- [x] No hardcoded values where an object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects
- [x] Property comparisons use original object

## Manual Verification

- [x] Feature manually tested end-to-end (via Playwright against the built `rela-server` binary + embedded SPA)
- [x] Each acceptance criterion verified
- [x] Edge cases verified in unit tests

**Verification Evidence:**

Frontend gates (in `frontend/`):
- `npm run typecheck` → clean (vue-tsc, 0 errors).
- `npm run lint` → 0 errors (89 pre-existing warnings, none in changed files).
- `npx prettier --check` on all 4 changed frontend files → clean.
- `npm run test:run` → **74 files, 1183 tests passing**, including the 24 new
ones (verbose run confirms each).

e2e (in `e2e/`, real `rela-server` binary + embedded prod SPA):
- `list.spec.ts › Filtering` group → **5/5 passing**, including the new
`relation filter narrows the list by target title`:
  - tasks list renders the `implements` relation control as a `<select>` whose
options are feature display titles (asserts `User Authentication` present),
  - selecting `User Authentication` narrows to TASK-001 ("Write unit tests") and
hides TASK-002 ("Refactor auth module") — proves title-as-value against the real
backend title-match (AC3),
  - clearing restores the full list (AC4),
  - property-filter tests in the same group still green (AC5 no regression).

Acceptance-criteria mapping:
- AC1 select ≤10 → `FilterBar.test.ts` "renders select mode at or below the threshold"; e2e `<select>`.
- AC2 typeahead >10 → "renders typeahead mode above the threshold (11 candidates)".
- AC3 value = bare `_title`, list narrows → `EntityTargetSelect` "option VALUE is the bare title, not 'Title (ID)'" + e2e narrowing.
- AC4 empty clears → "empty commit clears the filter (omits the key)" + e2e clear.
- AC5 property filters unchanged → "property filters still render as before" + e2e Filtering group green.
- AC6 direction → "incoming fetches from `from`" / "outgoing fetches from `to`".
- AC7 deep-link → "a deep-linked relation filter value is passed through" + "deep-link value binds directly as the selected option".

## Quality

- [x] Code follows project patterns (reuses `entityDisplayTitle`, `isCancelledFetch`, `entitiesStore.fetchList`, existing FilterBar select machinery; `EntityTargetSelect` in `common/` per frontend conventions)
- [x] Checked for DRY opportunities — `TargetCandidate` shared type extracted to `types/entity.ts`; threshold/limit as named constants; no premature abstraction of RelationPicker (kept additive per plan)
- [x] No security issues introduced (option labels via `{{ }}` only, no `v-html`; ACL-gated read endpoint reused; no new trust boundary)
- [x] No silent failures (cancelled fetch is the one deliberately-suppressed case, documented; other errors surfaced)
- [x] No debug code left behind

**Design-review findings all addressed in code:**
- RR-3MDVZD (wire `filter[<rel>]`) — value flows through unchanged `localFilters[relation]` → bracket-form plumbing; e2e proves the real wire works.
- RR-X4QWBF (bare `_title`) — `EntityTargetSelect` commits `entityDisplayTitle`, label-only uses `entityDisplayTitleWithId`; unit-tested.
- RR-NH8B6D (search vs committed) — `searchQuery` is component-local; unit test "external modelValue change does not clobber typing" + "partial search never committed on click-away".
- RR-A51QQ2 (deep-link + option values are titles) — `<select>` option values are bare titles; unit + e2e.
- RR-3TJVQJ (hyphen relation names) — documented limitation in `docs/data-entry.md`; in-repo e2e relation `implements` is identifier-safe.
