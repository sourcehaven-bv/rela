---
id: IMPL-BYME5I
type: implementation-checklist
title: 'Implementation: Condition-hidden field is pruned on save and can never be revealed again (silent data loss)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written — `frontend/src/composables/useHiddenFieldPolicy.test.ts`,
22 tests. This closes the coverage gap the analysis identified: there was **no**
unit test for the hide/reveal logic at all, which is why the interaction went
unnoticed.
- [x] Integration tests written — 6 new e2e tests in `e2e/tests/wizard.spec.ts`
covering the round-trip, reload persistence, and all three `clear_when_hidden`
values including the confirm-decline revert.
- [x] Feature implemented — see the 9-step plan on BUG-FB0LN8.
- [x] Edge cases handled — boolean `false` treated as a real value
(`isClearedForType`, so checkboxes don't prompt spuriously); empty string / null
/ empty array treated as nothing-at-stake; re-entrant trigger while a dialog is
open ignored rather than queued; post-flush watcher handled with a generation
counter.

## Manual verification (REQUIRED)

- [x] **Tests proven to catch the bug.** Stashed only `DynamicForm.vue`,
rebuilt the SPA + server, and re-ran the new e2e tests against the pre-fix
component: **5 of 6 failed**. The one that passed (`clear_when_hidden:yes`) is
the test that asserts the OLD destructive behavior, which correctly still holds
under the opt-in. Restored and re-ran: all 6 pass. The tests are not vacuous.
- [x] Each acceptance criterion verified end-to-end against the real
`rela-server` binary and a real browser (Playwright/chromium):
  - Hide a branch → stored value survives (`edit mode KEEPS a field…`)
  - Reveal it again → field re-renders **with its value**
(`a hidden field reappears…`) — the half nothing covered before
  - Survives a full page reload (`…survives a page reload`)
  - `clear_when_hidden: yes` → clears (old behavior, now opt-in)
  - `clear_when_hidden: confirm` + decline → value kept AND the triggering
change reverted (`done` back to `true`)
  - `clear_when_hidden: confirm` + accept → clears

## Quality

- [x] `go build ./...` clean; `go test ./internal/...` all pass
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK, no warnings
- [x] `just plimsoll` (god-object lint) — clean
- [x] `just coverage-check` — PASS (total 76.8%, package floors satisfied)
- [x] `npm run typecheck` (vue-tsc) — clean
- [x] `npm run lint` — 0 errors (91 pre-existing warnings, none added by these
changes; the one warning my test file introduced was fixed)
- [x] `npm run test:run` — 88 files, **1451 tests pass**
- [x] Full e2e suite — **236 passed**, 0 failed, 8 skipped (pre-existing skips)
- [x] No silent failures — the confirm path surfaces a real dialog; the config
enum is allowlist-validated so a typo is a build-time error, not a
silently-destructive default.

## Regressions explicitly checked

- [x] **RR-U9ERK** error-clearing effect preserved — `hiding an errored step
clears its flag and summary` still passes. It shared the watcher with the
destructive branch; only the destructive half was removed.
- [x] **RR-O4SRG** create-path prune intact — `a revealed-then-hidden field is
NOT persisted on create` and `submits a wizard and drops hidden-branch values`
both still pass. Retention lives in its own ref, so `formData` keeps its exact
prior meaning and the create path is untouched.
- [x] **Relation prune** intact — `a relation under a revealed-then-hidden step
is NOT persisted` still passes.
- [x] **RR-FD1B** read-only row invariant preserved — added
`computeVisibleFieldAffordances` so cards/list rows keep the stricter
closed-world shape (`keys(_fields) ∩ hidden == ∅`). The tombstone is emitted
only on the edit-path surface that actually needs it.
- [x] **ACL value secrecy** unchanged — the tombstone discloses the field NAME
only; `properties` still omits the value. `TestV1Affordance_PatchEcho_
StripsHidden` was tightened to assert the VALUE never appears anywhere in the
echo, rather than relying on a substring match on the key.

## Notes

- Settings e2e `shows forms count` updated 9 → 11 for the two new fixture forms.
- `DynamicForm.vue` grew 1886 → 1999 lines. The policy mechanism itself landed
in its own composable per RR-SH85S4; what remains in the component is wiring
plus the watcher rewrite.
