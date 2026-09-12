---
id: IMPL-X3OFMR
type: implementation-checklist
title: 'Implementation: Create forms need a "Create & add another" button for repeated entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

19 unit tests (`DynamicForm.addanother.test.ts`), 3 Go config tests
(`internal/dataentryconfig/validate_test.go`), 3 e2e tests
(`e2e/tests/create-add-another.spec.ts`) driving a real server and a real
create, which is the only level that shows TWO DISTINCT entities reaching the
store with the right values.

Error handling — two paths deliberately made louder on this action, because the
user no longer lands on the entity and so cannot see a problem for themselves:

- A create failure leaves the form untouched (values intact, no success toast,
  no reset) — pinned by the AC-8 test.
- A swallowed auto-link failure (`console.warn` only, on the navigate path)
  now raises a toast in `'again'` mode. Under repeated entry the old behaviour
  would produce N silently-unlinked entities.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

AC-2 is the notable one: rather than two tests comparing against duplicated
literals (which drift), it drives BOTH buttons in one test against the same
filled form and asserts `expect(callB).toEqual(callA)` — an actual test of the
"the two buttons cannot diverge" invariant (RR-7J1IHR).

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Automated, against a real `rela-server` + built SPA bundle (`just
build-frontend-e2e && just build-server-e2e`, then Playwright):

| AC | Verified by | Result |
|----|-------------|--------|
| 1 create-only / not-edit / not-embedded | 3 unit tests | PASS |
| 2 identical payload to primary Create | one-test two-button comparison | PASS |
| 3 does not navigate | unit (`router.push` not called) + e2e (`toHaveURL`) | PASS |
| 4 clean reset + query pre-fills re-applied | 2 unit tests | PASS |
| 4b `keep_on_add_another` carries over, unmarked does not | 3 unit tests + e2e | PASS |
| 4c config key round-trips on field AND relation | Go `TestValidateConfig_KeepOnAddAnotherRoundTrips` | PASS |
| 4d incoming picker cannot keep its selection | unit (mount-count) | PASS |
| 5 second create from same form instance | unit + e2e (two distinct ids) | PASS |
| 6 toast names the created id | unit | PASS |
| 7 not left dirty, incl. entry-locked field | unit (mounted, reads `isDirty`) | PASS |
| 8 failure does not reset or claim success | unit | PASS |
| 9 wizard returns to step 1 | unit | PASS |

Full suites after the change: **2355 frontend unit tests pass**, **284 e2e pass
(0 fail)**, Go `dataentryconfig` + `dataentry` pass, `vue-tsc` clean, `eslint`
0 errors, `just arch-lint` OK, `just comment-lint` OK.

**Mutation-verified** (the assertions above were checked to actually FAIL when
the fix is removed, so they are not passing vacuously):

| Mutation | Tests that failed |
|----------|-------------------|
| drop the kept-value re-apply | 3 carry-over tests |
| don't release `createdEntityId` | 3 tests incl. AC-5 |
| don't bump `saveGeneration` | AC-4d picker remount |
| re-baseline `dirty` before the dry-run instead of after | AC-7 locked-field test |
| skip `wizard.goTo(0)` | AC-9 |
| remove `pointer-events: none` from the toast container | 2 e2e tests |

**Two real defects were found by this verification, not by review:**

1. **The success toast covered the button** (`Toast.vue`: `position: fixed;
   bottom/right` — exactly where the form actions sit). Harmless while every
   create navigated away; with the user kept on the form it swallowed the click
   for the next record entirely. Fixed with `pointer-events: none` on the
   container and `auto` on each toast (its dismiss button stays clickable).
   jsdom has no hit-testing, so **no unit test could have caught this** — it
   took the e2e.
2. **`FormPage.submitButton` became ambiguous.** Its `has-text("Create")` clause
   is a substring match, so the new button matched it too and every submit in
   the e2e suite became a strict-mode violation. Narrowed to `text-is()`.

One planning assumption was also **disproved during implementation** and the
code corrected to say so: RR-ZYU2GC's `hiddenPolicy.releaseAll()` hazard does
NOT apply on the create path — `useChangePolicy` is gated on
`enabled: () => isEdit.value`, so the retention map is never populated in create
mode (confirmed: removing the call changes no test). The call is kept as
belt-and-braces with a comment stating exactly that, rather than left implying
it guards something it doesn't.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: `PendingButton` for the action (as the primary Create does);
the `clear_when_hidden` allowlist+coherence shape for the new config key; the
component-scoped sr-only clip from `AutoSaveIndicator` rather than inventing a
global; `applyTemplate` reused rather than a second template path.

DRY: the two outcomes share ONE `handleSubmit` body and branch at exactly two
points (the latch, the terminal step), so the payloads cannot diverge — that is
the point of the `mode` parameter rather than a second function. The blank-form
sequence is `resetCreateForm()`, so `onMounted` and the reset cannot drift.

Security: no new server surface and no new request input. The one new input is
operator-authored config (`data-entry.yaml`), validated at load, which cannot
widen anyone's access — it only decides whether a value the user already typed
stays in their own form. Each create remains an independently authorized POST;
N creates from one form instance are indistinguishable server-side from N
creates from N page loads.

No debug code: verified `grep DBG` clean, temporary debug specs deleted, the one
test seam added (`_originalData` on `defineExpose`) is documented as such and is
read-only.
