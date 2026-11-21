---
id: PLAN-CAJLN42
type: planning-checklist
title: 'Planning: Portal UX fixes: date picker and mobile menu bar and select-all label and phone list columns'
started: "2026-11-20"
completed: "2026-11-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: an `as_of` entry in `builtins:` for the sidebar date picker, its
own row in the sidebar header and a "Now" label; the mobile hamburger in an
opaque sticky bar; the select-all label for an unsectioned list; a list column
`primary:` that overrides the compressed-layout default. Out: other findings of
the same test (handled in other PRs), card layouts, any change to what the as-of
read path allows.

**Acceptance Criteria:**
1. `builtins: {as_of: {hidden: true}}` removes the picker; other rows unchanged (Go handler test, Sidebar vitest).
2. The picker takes its own row and shows "Now" that opens the browser picker (AsOfPicker vitest, manual check at 1400px).
3. At 375px the hamburger does not overlap a banner or content (manual check on dashboard, list and entity pages; existing mobile-layout e2e).
4. The unsectioned select-all label reads "Select all" / "Alles selecteren" (unit test).
5. `primary:` overrides the default either way; refused on card and view columns (EntityList vitest, dataentryconfig test).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: four small fixes on existing patterns)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: config and CSS inside rela)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows rela's own builtins and busy column patterns)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `dataentryconfig.Builtins` and `hidden_builtins` (TKT-JEAR2UA) already gate pinned rows; `as_of` is one more name.
- `refuseListOnlyColumn` (busy_column.go) already refuses list-only column keys on cards and view sections.
- RlTable `compact="compress"` keeps columns marked `primary`; EntityList derives it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Add `BuiltinAsOf` to the row allowlist; Sidebar renders
the picker only once `hidden_builtins` is known and `as_of` is not in it.
AsOfPicker becomes a full-width flex row with a "Now" button over the empty
input that calls `showPicker()`. App.vue wraps the hamburger in a sticky bar
that pulls itself over the pane's top padding (`--pane-padding-top`). The
select-all message drops "in" for an empty section. `ListColumn.primary *bool`
on the server and `primary?: boolean` in the SPA; `isPrimaryColumn` prefers it.

Alternatives: a separate `as_of:` config key (rejected: `builtins:` already
means "hide or gate a built-in sidebar control"); hiding the picker with CSS on
phones (rejected: the operator wants it off per space); choosing phone columns
by width priority numbers (rejected: one boolean covers the need).

**Files to modify:**
internal/dataentryconfig/{builtins.go,config.go,busy_column.go},
internal/apiwire/v1/responses.go, frontend/src/{App.vue,types/config.ts,
components/common/{Sidebar.vue,AsOfPicker.vue},components/lists/EntityList.vue,
i18n/locales/*.json}, rela-components useMessages and messages.nl, docs guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** data-entry.yaml (operator): `as_of` is checked
against the builtins allowlist; `primary` is a bool, refused outside lists.

**Security-Sensitive Operations:** None. Hiding the picker is a UX filter;
`?as_of=` reads keep their own ACL. Config is not a secret.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria; manual checks in a local
harbourmaster copy at 375px and 1400px.

**Edge Cases:** whitespace-only section name; browser without `showPicker`;
picked date (no Now button); `primary: false` on an enum column; topbar page
with and without a banner.

**Negative Tests:** `primary` on a dashboard card or view section column fails
to load; unknown builtins row still fails.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** The sticky bar changes mobile layout on every page: checked on
dashboard, list and entity pages and with the existing mobile-layout e2e.
Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs-project GUIDE-data-entry (builtins table, column
options, columns on a phone), regenerated into docs/data-entry.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small fixes on existing patterns; cranky-code-reviewer runs in review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
