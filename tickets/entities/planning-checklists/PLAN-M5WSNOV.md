---
id: PLAN-M5WSNOV
type: planning-checklist
title: 'Planning: Natural-language date entry in date fields'
started: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: a language-neutral phrase parser with per-language vocabulary packs (en, nl), a text box in `DateWidget` with a live preview, the todo-app example. Out: datetime/times of day, recurrence phrases, server-side parsing, config keys.

**Acceptance Criteria:**
1. "tomorrow" / "morgen" resolve to today+1 (unit test with fixed today).
2. Weekday, next weekday, in N units, N units ago, next week/month/year, day-month(-year) and ISO resolve (table test per pack).
3. The widget shows the resolved date before applying; Enter or blur applies it; an unknown phrase shows a hint and changes nothing (component test).
4. A new pack needs no parser change: the test suite runs the same shape table over every registered pack.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small frontend change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: fixed phrase grammar; the libraries surveyed cover the reference behaviour)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
chrono-node and sugar.js were considered and rejected: large bundles, English-first, and locale coverage outside our control. `utils/format.ts` formats the preview; `i18n/locale.ts` gives the active language.
<!-- Document what you found:
- Libraries considered (with pros/cons, why chosen or rejected)
- Similar patterns in codebase (file:line references)
- Reference implementations that inspired the approach
- Relevant concepts from rela-docs or rela-issues-and-design-tickets
-->

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
`utils/naturalDate/parse.ts`: normalise (lowercase, NFKC, strip diacritics, collapse spaces), then try shape matchers built from a pack (`relativeDays`, `weekdays`, `months`, `next`, `this`, `in`, `ago`, `units`). Pure date arithmetic on local Y/M/D, no Date-string parsing. Month overflow clamps to the month's last day. `locales/index.ts` maps a language code to a pack; `packFor(lang)` falls back to `en`.

**Files to modify:**
frontend/src/utils/naturalDate/*, frontend/src/widgets/DateWidget.vue, i18n en/nl.json, docs-project GUIDE-data-entry, examples/todo-app README.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
Threat model: the only input is text the signed-in user types into their own form. It never leaves the browser except as the ISO date the form already saves, which the server validates as today. The parser uses anchored regexes with no nested quantifiers (no ReDoS) and caps input at 64 characters. Unknown input returns null and the value is unchanged. The preview renders through Vue text interpolation, so no HTML injection.

**Security-Sensitive Operations:**
None. No ACL, storage, network or config surface changes.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
Vitest table tests per pack with a fixed today (criteria 1, 2, 4); DateWidget component test (criterion 3).

**Edge Cases:**
Empty input, whitespace, 31 Jan + 1 month, 29 Feb in a non-leap year (rejected), "next friday" on a Friday, mixed case and diacritics, over-long input, year rollover for day-month without year (next occurrence).
<!-- List specific edge cases and expected behavior. Consider:
- Empty/null/missing values
- Boundary values (0, -1, MAX_INT)
- Special characters, unicode, null bytes
- Concurrent access
- Resource exhaustion
-->

**Negative Tests:**
"31 february", "in -3 days", "in 99999 days" (capped), gibberish: all return null.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
Ambiguous phrases ("this friday" on a Friday) are documented: "this" is today or later, "next" is strictly after. Effort: s.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
docs-project GUIDE-data-entry (date fields section).
<!-- Which docs need updating? Check all that apply:
- [x] docs/metamodel.md - New metamodel features
- [x] docs/cli-reference.md - New/changed commands
- [x] docs/data-entry.md - UI changes
- [x] CLAUDE.md - New patterns or conventions
- [x] README.md - Project-level changes
- [x] N/A - Internal change, no user-facing docs needed
-->

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-SEK0GV5, RR-DY3S20J, RR-M81OQYC (all minor, addressed in the approach)
