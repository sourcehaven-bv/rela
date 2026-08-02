---
id: IMPL-PQ0TUL
type: implementation-checklist
title: 'Implementation: History/diff views: put selected versions in the URL so a diff is shareable'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**What was built:**

`frontend/src/composables/useVersionSelectionSync.ts` — one composable owning
the URL representation of the compared pair, wired into both history views.
Follows the house seed/replace/echo pattern (`useUrlFilterSync`,
`useFormWizard`'s `?step=N`), with two-phase seeding because the valid-version
allowlist doesn't exist until `listVersions` resolves.

| File | Change |
|---|---|
| `frontend/src/composables/useVersionSelectionSync.ts` | New — parse/serialize/seed/write/echo-guard. |
| `frontend/src/composables/useVersionSelectionSync.test.ts` | New — 39 unit tests. |
| `frontend/src/views/HistoryView.vue` | Composable wired in; `load(fromUrl)`; single write path; `:data-version` added to timeline rows. |
| `frontend/src/views/RelationHistoryView.vue` | Same; `defaultSelection()` extracted so the post-restore reset and the composable's `defaults()` can't drift. |
| `e2e/pages/history.page.ts` | New — entity-history page object. |
| `e2e/pages/relation-history.page.ts` | URL/selection helpers. |
| `e2e/pages/index.ts` | Export both history page objects. |
| `e2e/tests/history-url-params.spec.ts` | New — 5 postgres-gated e2e tests. |
| `e2e/tests/fixtures.ts` | `waitForEntityVersions` added; polling loop shared with the relation waiter via `pollForVersions`. |
| `docs/postgres-backend.md` | Shareable-diff URLs documented for entities and relations. |

**Deviations from plan:** none material. Docs landed in
`docs/postgres-backend.md` rather than `docs/data-entry.md` — that's where the
history feature is actually documented (`data-entry.md` never mentions it), so
the new text sits directly under the paragraph describing the history UI.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

E2E entity ids come from the server-assigned `e.id` (features use sequential
ids, so a literal `FEAT-001` would be fragile) via a `featureWithHistory(api,
title, versions)` helper that also waits out the async capture sweep. Unit tests
are `it.each`-tabulated for the 14 malformed-input cases.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Executed against a real PostgreSQL backend (`rela-server-postgres` built from
this branch, `RELA_E2E_DATABASE_URL` + the CI sweep intervals), not mocks.

| AC | Result | Evidence |
|---|---|---|
| 1 deep link | PASS | e2e "deep link selects the pair…" — `?base=1&target=2` yields select values `1`/`2`, caption `v1 → v2`, timeline row 1 `.selected`, no interaction. |
| 2 writes URL | PASS | e2e asserts URL after dropdown change, timeline click (`base=2`), and swap (`3`/`2`); unit tests assert the `router.replace` payload. |
| 3 reload | PASS | e2e picks `1`/`3`, reloads, pair survives. |
| 4 `current` round-trips | PASS | Unit: parses to the sentinel, serializes back unchanged, never coerced to a number. e2e: bare URL publishes `target=current`. |
| 5 bare URL = today's defaults | PASS | e2e: bare URL → `base=3&target=current` (newest→current) and the URL gains explicit params. |
| 6 bad params degrade | PASS | e2e `?base=999&target=nonsense` → defaults, no `.error-state`. Unit: 14 cases incl. `abc`, `-1`, `0`, `1.5`, `1e2`, `999`, `../../etc/passwd`, `<script>`, arrays, empty. |
| 7 no history spam | PASS | Unit asserts `replace` called, `push` never. |
| 8 restore resets | PASS | `load(false)` on the restore path; pre-existing relation-history restore e2e still passes. |

Plus an unplanned AC: selection writes **merge** into the query — e2e asserts a
`?return_to=/list/features` survives a selection change.

**Mutation-tested the tests.** Rather than trust green, I injected the exact
regression the plan flagged as highest-risk (numeric side returned as a *string*
— the `v-model` type trap) and confirmed both layers fail: 8 unit tests red, and
the e2e deep-link test red on the timeline-highlight assertion. Then reverted
and re-confirmed green. The tests detect the failure they were written for
rather than passing vacuously.

**Full suites:** frontend unit `1423 passed (87 files)`; e2e `238 passed`
(includes the 2 pre-existing relation-history tests and the whole wizard suite —
the closest sibling of this URL-sync pattern — so no regression). `npm run
typecheck` clean in both projects; `npm run lint` 0 errors in both;
`markdownlint` clean on the changed doc; prettier clean on all changed files
(the repo's 123 pre-existing format warnings are untouched and none are mine).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**DRY:** the composable exists precisely so the ~40 lines of sync logic aren't
duplicated across two already-near-duplicate views. Two further extractions
during implementation: `defaultSelection()` in the relation view (the default
pair is needed both by `defaults()` and by the post-restore reset — inlining it
twice would let them drift), and `pollForVersions` in `fixtures.ts` (the new
entity waiter would otherwise have been a verbatim copy of the relation one).

**Security:** the allowlist is the load-bearing control — a side is accepted
only if it is `current` or an ordinal the *server returned*, so a crafted value
never reaches `getVersion()` or the DOM. Explicitly unit-tested with traversal
and script-tag payloads. No new authorization surface: the params only choose
among versions the caller could already read, and the read gate (entity
visibility; relations gated on both endpoints) is untouched server-side.

**Two defects found and fixed during implementation**, both caught by tests
rather than review:

1. *Spurious recompute on unrelated navigation.* The route watcher's getter
returned a fresh array `[base, target]` each run, so Vue's reference comparison
fired it on any query change — a `?q=` edit elsewhere would re-diff. Fixed by
watching a joined string; the failing unit test ("ignores changes to unrelated
query params") now pins it.
2. *Wrong route segment in the new e2e spec* — used the plural (`features`)
where the SPA route takes the singular (`feature`), producing "Entity not
found". Caught by actually running the suite against postgres.
