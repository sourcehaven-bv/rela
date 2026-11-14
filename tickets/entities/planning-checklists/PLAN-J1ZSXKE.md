---
id: PLAN-J1ZSXKE
type: planning-checklist
title: 'Planning: Workshop step mode for pages (mode: steps, present mode)'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: `mode: steps` on a page, `prompt:` on a tab of a steps page, a stepper
bar with Back/Next, present mode (`?present=1`) that hides the sidebar and the
next-action card and enlarges the content, a full screen button, PageDown/PageUp
to move between steps, docs and tests.

Not in scope (follow-up tickets): a one-at-a-time step over a set of entities,
document/dashboard tabs on entity pages, `done_when` per step, timebox, kanban
quick-add, creating a column from the board, coloured matrix cells, phones,
voting, guests, shared step state between screens.

**Acceptance Criteria:**

1. A page with `mode: steps` loads; its tabs render as numbered steps with
the current one marked (`aria-current="step"`), and no tab bar. Test: PageView
unit test; Go config test for a valid steps page.
2. Next opens the following step and Back the previous one; Back is disabled
on the first step and Next on the last. Each step has its own URL, so the
browser's back button returns to the previous step. Test: PageView unit test
with the router; e2e in pages.spec.ts.
3. `prompt:` shows as a large heading above the step's view.
Test: PageView unit test.
4. Present mode: `?present=1` on a steps page hides the sidebar and the
next-action card and enlarges the content; the flag stays on when moving between
steps; "Exit" removes it. On a page without `mode: steps` the flag does nothing.
Test: PageView + App/RlAppShell unit tests; e2e.
5. In present mode, PageDown/PageUp move to the next/previous step, except
when focus is in an editable field or a modal is open. Outside present mode the
keys scroll as usual. Test: PageView unit test.
6. Config errors: an unknown `mode`, a `prompt` on a page without
`mode: steps`, a prompt over 280 characters or with control characters all fail
to load with a message naming the page and tab. Test: table test in
page_test.go.
7. The sidebar payload carries `mode` and each tab's `prompt`.
Test: views_handler test.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the approach extends an existing feature (Pages).

**Existing Solutions:**

- Pages and entity pages (`internal/dataentryconfig/page.go`,
`frontend/src/views/PageView.vue`): tabs, scoping, URLs per tab. Steps reuse all
of it; only the rendering differs.
- Wizard forms (`steps:` on forms) use `?step=N`; here each step already has a
path segment, so no query is needed.
- `usePageFrame` (module-level slot read by App.vue) is the pattern for a
view telling the shell something; present mode uses the same shape.
- No library needed: the Fullscreen API is native.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Backend:
- `Page.Mode` (`tabs` default, `steps`), added explicitly to the sidebar
wire type `SidebarPage` (mode is not header content).
- `PageTab.Prompt`; copied to `SidebarPageTab.Prompt`.
- `validatePages`: unknown mode; prompt only on a steps page; prompt at most
280 characters, no control characters.

Frontend:
- Types: `mode` on PageConfig/SidebarPage, `prompt` on tabs.
- `PageView`: in steps mode the frame gets no tabs; a new `PageSteps.vue`
above the view draws the numbered step list (links), Back/Next, Present / Exit
and Full screen buttons, and the prompt heading.
- `usePresentation` composable: module-level state. PageView publishes a
computed of the route and the page mode (true only for a steps page with
`?present=1`), so browser back out of present mode restores the shell; cleared
on scope dispose. App.vue reads it: RlAppShell gets a new `hideSidebar` prop,
the hamburger and NextActionCard are not rendered, and the frame gets a
`--presenting` class that zooms the main pane.
- Step links carry `?present=1` while presenting; other query state is
dropped as tab switches do today.
- PageDown/PageUp listener in PageView in present mode only; ignored when
`isInputFocused()` or `isAnyModalOpen()`.

Alternatives rejected: a new top-level `sessions:` key (duplicates pages); flow
states that name views (flows track a subject over days, not rounds of minutes);
a custom app (cannot embed rela's views).

**Files to modify:**

- internal/dataentryconfig/page.go, page_test.go
- internal/apiwire/v1/responses.go
- internal/dataentry/views_handler.go (+ test)
- frontend/src/types/config.ts
- frontend/src/views/PageView.vue, PageView.test.ts
- frontend/src/components/pages/PageSteps.vue (new) + test
- frontend/src/composables/usePresentation.ts (new)
- frontend/src/App.vue
- frontend/packages/rela-components/src/components/layout/RlAppShell.vue
- frontend/src/i18n/locales/en.json, nl.json
- e2e/tests/pages.spec.ts (+ fixture config)
- docs-project/entities/guides/GUIDE-data-entry.md (regenerates docs/data-entry.md)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `mode` and `prompt` come from data-entry.yaml (operator config, not
secret). Mode is an allowlist; prompt is length- and character-checked at load
and rendered as text (no v-html).
- `?present=1` is a client-only display flag. It changes no request and no
permission.

**Security-Sensitive Operations:**

None. Step views are the existing tab views with their existing gates; a tab the
principal may not see is not a step (the sidebar payload already filters tabs).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the test named under each acceptance criterion.

**Edge Cases:**

- A steps page with one visible step: Back and Next both disabled.
- A step hidden by `permission:`: numbering follows the visible steps.
- `?present=1` on a tabs page: ignored.
- Leaving the page while presenting: the sidebar comes back (state cleared
on dispose).
- Full screen refused by the browser: no error, button stays.
- Empty prompt: no heading.

**Negative Tests:** unknown mode, prompt on a tabs page, prompt over 280
characters, prompt with a control character; PageDown in a text field does not
navigate.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- CSS `zoom` on the main pane could disturb drag-and-drop coordinates.
Mitigation: verify kanban drag in present mode manually; fall back to a
font-size scale if it misbehaves.
- The stepper bar takes vertical space from full-height views (kanban,
gantt). Mitigation: keep it one line; check in the browser.

Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/data-entry.md (via docs-project GUIDE-data-entry): Pages section

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-VE1MC0U (keys only in present mode), RR-7OXG9QK
(Mode on Page), RR-MJGVME2 (verify drag under zoom), RR-K2S9940 (present state
follows the route); all addressed above.
