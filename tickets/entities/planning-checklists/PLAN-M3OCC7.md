---
id: PLAN-M3OCC7
type: planning-checklist
title: 'Planning: Translate end-user screens of the data-entry SPA to Dutch'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: the end-user screens of the data-entry SPA (`frontend/src`): lists,
forms (incl. wizard), entity detail and history, navigation and switchers,
search, dashboard, kanban, calendar, gantt, piles, comments, channels, actions
and flows, next actions, toasts and confirmations, empty and error states,
conflicts and analysis screens. Template text, `aria-label`, `title`,
`placeholder`, literals inside bindings, and user-facing TS messages (toasts,
confirm dialogs, store errors).

Out of scope: the Configure space (`components/configure`, `views/configure`)
and Settings internals (`SettingsView.vue`, `components/settings`) — that is
TKT-2TUDZH. Operator text from the configuration stays untranslated.
`rela-components` already has its own Dutch set (TKT-ZL1QAO). Server-side
messages are not changed.

**Acceptance Criteria:**

1. Every in-scope file is off the bare-string allowlist, or its count is lower;
the allowlist totals (`MAX_LISTED_FILES`, `MAX_LISTED_STRINGS`) go down. Test:
`guards.test.ts` passes; the remaining entries are Configure/Settings.
2. Every new key is in `en.json` and `nl.json` with the same placeholders and
plural forms, and is used. Test: `guards.test.ts` key-parity and unused-key
checks.
3. With the interface set to Dutch, the in-scope screens show Dutch text.
Test: manual check in the browser with the language set to Nederlands; the two
e2e specs pinned to `nl-NL` run against Dutch text.
4. English text is unchanged. Test: unit tests and the e2e suite pass without
changing English expectations (except where selectors move to roles/test ids).
5. Dutch uses "je", plain wording, and a consistent glossary. Test: glossary in
the PR body for review.

## Research

- [x] For larger features: run `/research` to create a structured research doc (N/A: the mechanism is settled by TKT-ZL1QAO; this is extraction work)
- [x] Searched for existing libraries that solve this problem (N/A: vue-i18n already chosen in TKT-ZL1QAO)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects (N/A: following the in-repo pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `Sidebar.vue`, `AccountMenu.vue`, `SidebarFooter.vue`, dashboard cards: the
converted slice to copy.
- `src/i18n/index.ts` (`t` outside setup), `LanguageSetting.vue` (literal
key records), `guards.test.ts` (ratchet, parity, unused keys).
- `rela-components/messages.nl.ts`: existing Dutch terms (Opslaan,
Annuleren, Verwijderen, Kanaal, Stapel in `nl.json`) to stay consistent.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- Shared verbs live under `common.*` (save, cancel, delete, ...). Screen
text lives under one top-level area per screen group (`entity.*`, `form.*`,
`comments.*`, `list.*`, `search.*`, `calendar.*`, `gantt.*`, `piles.*`, ...).
- Components use `useI18n()`; composables and stores import `t` from
`@/i18n` and call it when the message is built, not at module load.
- Counts use vue-i18n plural forms (`{n} item | {n} items`); sentences with
embedded values use named placeholders, never string concatenation.
- Value→text maps use literal records so the unused-key check sees each key.
- `bare-strings.json` counts are recomputed after conversion; entries at
zero are removed; `MAX_LISTED_*` are lowered to the new totals.
- Two stacked PRs by screen area, to keep review size manageable:
PR 1 = entity detail, history, forms and wizard, editor, widgets, comments,
channels, `common.*`. PR 2 = lists, search, dashboard, kanban, calendar, gantt,
piles, actions/next actions, navigation and switchers, toasts, confirmations,
conflicts, analysis.
- e2e: specs keep running in English (default browser locale). The two specs
pinned to `nl-NL` get role/test-id selectors where they select English.

Alternatives: split the locale file per area (rejected: changes reviewed
infrastructure for no user gain); one PR (rejected: several thousand lines of
mechanical diff is hard to review alongside the Dutch wording).

**Files to modify:** `frontend/src/i18n/locales/{en,nl}.json`,
`frontend/src/i18n/bare-strings.json`, `frontend/src/i18n/guards.test.ts` (MAX
constants), the in-scope `.vue` files under `src/components`, `src/views`,
`src/widgets`, the message-building composables and stores (`useBulkDelete`,
`usePiles`, `useConfirm`, `useAutoSave`, `useEntityPageEditing`,
`useToolbarActions`, `stores/ui.ts`), unit tests that assert on changed text,
`docs/data-entry.md` (which screens are translated).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Messages are static JSON shipped in the bundle. Placeholders carry entity titles
and labels; vue-i18n escapes interpolation in `t()` output and templates render
text, not HTML. No `v-html` with translated text is added.

**Security-Sensitive Operations:** none. No API, ACL or storage change.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria. Unit: `npm run test:run` (guards,
existing component tests in English), `npm run typecheck` (typed keys), `npm run
lint`. Integration: full e2e suite in CI; locally the specs for the converted
screens and the `nl-NL` specs.

**Edge Cases:**

- Plurals for 0/1/n counts (Dutch and English).
- Placeholders containing operator labels in either language.
- Text built in a store (toast) keeps its language after a switch (accepted
per frontend/CLAUDE.md).
- Dutch strings are longer: check buttons and narrow columns do not break.

**Negative Tests:** a misspelt key fails `vue-tsc`; a missing `nl` key fails key
parity; a new bare string fails lint (existing guards).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- e2e selectors on English text break → default locale stays English; fix the
`nl-NL` specs with role/test-id selectors.
- Unit tests asserting English text → test setup installs i18n, English is the
default, so they keep passing.
- A `t()` call at module load freezes the language → review checks for it.
- Wording quality → glossary in the PR for review.

Effort: xl.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/data-entry.md - which screens are translated
- [x] ~~CLAUDE.md~~ (N/A: no new conventions)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see review-responses linked to the ticket.
