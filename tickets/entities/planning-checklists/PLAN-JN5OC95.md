---
id: PLAN-JN5OC95
type: planning-checklist
title: 'Planning: Settings: search picker for relation defaults'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: drop the per-target-type read from `GET /api/v1/_settings`; send
`targetTypes` instead of `targets`; replace the relation-default select (global
and override rows) with a search picker; bound `EntityPickerModal`'s multi-type
search. Out: the other `scopedHeaders` callers in the unpaged-read allowlist
entry (TKT-D3RAB0E, TKT-I39Y7N4); a bare-id entity lookup endpoint.

**Acceptance Criteria:**
1. Loading settings costs the store no more than binding the principal, at any
store size (`TestQueryBudget_SettingsReadsNoEntities`).
2. The response carries `targetTypes` per relation and no `targets`
(`TestSettings_RelationDefsCarryTargetTypesOnly`, golden corpus).
3. The picker shows the saved target's title from one entity read and searches
the target types with limit 20 (`RelationDefaultPicker.test.ts`).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: reuses an in-repo picker)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern exists)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `EntityPickerModal`
(frontend/src/components/forms/EntityPickerModal.vue) already searches
`/_search` with a type filter, debounce, abort and limit; it is used by the
markdown editor and channels. `listRecentlyModified` shows the `type:a,b` query
clause for several types.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Backend: `APIRelationDef` gets `targetTypes`; `Targets`,
`APIRelationTarget` and `settingsRelationTargets` go; `appearanceHandler` loses
its unused `services` and `viewReader`. Frontend: new `RelationDefaultPicker`
wraps `EntityPickerModal` (limit 20) and reads the saved target with
`getEntity`, trying each target type in order. `EntityPickerModal` gains
`maxResults` and sends several types as one `type:` clause with the limit
instead of an unlimited search filtered in the browser.

Alternatives: a bare-id lookup endpoint for the title (rejected: new API surface
for one caller; one read per target type is enough); an inline combobox
(rejected: the modal already handles keyboard, a11y and aborts).

**Files to modify:** internal/dataentry/settings_handlers.go,
appearance_handler.go, tests and golden files;
internal/archguard/unpagedread_allowlist_test.go (entry text);
frontend/src/api/settings.ts, views/SettingsView.vue,
components/settings/RelationDefaultPicker.vue (new),
components/forms/EntityPickerModal.vue, i18n locales; user guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The search text goes to `/_search`, which is
read-gated. Target type names come from the schema. The saved id is read through
the gated entity GET.

**Security-Sensitive Operations:** None new. Fewer: the settings response no
longer carries any entity data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria. Golden parity corpus covers the
wire shape per principal.

**Edge Cases:** Empty default (no read); several target types (tries each); a
saved id no type answers for (shows id and "not found"); a picked target (no
read).

**Negative Tests:** The budget test fails against the old handler (10 calls,
2043 rows padded).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Changing `EntityPickerModal`'s multi-type query affects the editor
and channel pickers; mitigated by keeping the client-side type filter and
updating its test. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-data-entry (and generated docs/data-entry.md):
one sentence on choosing the target by search.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: effort s, reuses an existing component; reviewed in code review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
