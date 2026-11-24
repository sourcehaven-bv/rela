---
id: PLAN-QDWTO3B
type: planning-checklist
title: 'Planning: Natural-language date input'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a text box beside every `date` widget in edit mode (forms, inline
edit, section edit). The user types a phrase in English or Dutch; Enter or
leaving the box turns it into an ISO date and sets the value. Both languages are
always accepted. Out: datetime properties, times of day, ranges, other
languages, server-side parsing, the filter bar.

**Acceptance Criteria:**
1. Typing `tomorrow` or `morgen` and pressing Enter sets tomorrow's date.
2. `next friday`, `volgende vrijdag`, `in 3 days`, `over 2 weken`,
`5 march`, `5 mrt 2027`, `5/3`, `end of month` resolve as documented in
`naturalDate.ts`.
3. An unknown phrase leaves the value unchanged, marks the box
`aria-invalid` and shows a translated message.
4. An empty box does nothing.
5. The native date picker still works and stays the first control, so
inline edit still opens the picker on click.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small, self-contained frontend change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: library survey covered it)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- chrono-node: English is solid, Dutch support is partial and the bundle is
large (about 100 kB). Rejected: one widget does not justify the weight.
- date-fns: no phrase parsing. Not needed: plain `Date` arithmetic in local
time is enough for day precision.
- Codebase: `src/widgets/DateWidget.vue` is the only date edit control;
`utils/format.ts` formats dates; `RlInlineEdit` focuses the first input and
calls `showPicker` when `autoOpenPicker` is set.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `frontend/src/utils/naturalDate.ts`: pure `parseNaturalDate(text, today)`
returning `YYYY-MM-DD` or null. Local calendar arithmetic; invalid dates (31
feb) return null; input over 64 characters returns null.
- `DateWidget.vue` edit mode: wrap the native input and a text input
(`class="rl-control"`, placeholder from i18n) in a span. On Enter
(preventDefault) or change, parse with today's date; on success emit the ISO
value and clear the box; on failure set `aria-invalid` and show the message in a
small element referenced by `aria-describedby`.
- i18n keys `widgets.date.natural.placeholder`, `.label`, `.invalid` in
en.json and nl.json.
- Alternative rejected: replacing the native input with one text box. It
loses the picker and the browser's locale display.

**Files to modify:**
- frontend/src/utils/naturalDate.ts (+ test)
- frontend/src/widgets/DateWidget.vue (+ test)
- frontend/src/i18n/locales/en.json, nl.json
- docs-project guide for data entry (date input section)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The only input is text the user types into their
own form. The parser is an allowlist of fixed word tables and anchored regular
expressions with no nested quantifiers, so there is no backtracking risk; input
is capped at 64 characters. Its output is either null or a string built from
numbers, which then goes through the existing PATCH path and the server's date
validation.

**Security-Sensitive Operations:** None. No new request, endpoint, storage or
rendering of HTML. Threat model: `.ignored/threat-TKT-ICADEGW.md`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- `naturalDate.test.ts`: table of phrases in both languages against a fixed
today (criteria 1, 2).
- `DateWidget.test.ts`: Enter on a phrase emits the ISO value; an unknown
phrase emits nothing and sets `aria-invalid`; empty does nothing; the native
input is the first input (criteria 1, 3, 4, 5).

**Edge Cases:** Month ends (31 jan + 1 month clamps to 28/29 feb), leap years,
year rollover for `5 jan` in December, weekday equal to today (`friday` on a
Friday means today, `next friday` a week on), mixed case and extra spaces.

**Negative Tests:** `31 feb`, `0/3`, `13/13`, `next banana`, an over-long
string, empty input.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Day/month order: `5/3` is read day first, which suits Dutch but not US
English. Mitigation: documented, and the result shows in the native input at
once so a wrong reading is visible.
- Inline edit commits on blur of the control: moving focus to the second
box must not end the edit. Mitigation: both inputs sit inside the edit slot, and
a test covers it.

Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - UI changes (via the data-entry guide in docs-project)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the review-responses linked to TKT-ICADEGW.
