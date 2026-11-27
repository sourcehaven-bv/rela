---
id: PLAN-EQCF6T9
type: planning-checklist
title: 'Planning: Options to hide type and ID on cards and mention lists'
started: "2026-11-26"
completed: "2026-11-26"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `hide_type` and `hide_id` on view sections (`list`, `cards`,
`content`; `hide_id` also on `nested`), and
`entity_views.<type>.mentioned_by_hide_id` for the built-in "Mentioned by"
panel. Out: kanban cards, list rows, form side panels (refused at load), and
changing the default.

**Acceptance Criteria:**
1. Without the keys, rows show type, title and ID as before. Test: EntityDetail.mentions.test.ts "shows the type label and ID by default".
2. With `hide_type`/`hide_id`, rows leave them off, and an untitled or blank-titled row keeps its ID. Test: same file, "leaves them off when asked".
3. `mentioned_by_hide_id` leaves the address off the panel rows. Tests: MentionedByPanel.test.ts and views_mentions_test.go.
4. Misplaced keys fail at config load. Test: validate_hideidentity_test.go.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small enhancement)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: rendering flag in existing components)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows the UI guidance principle 13)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Follows the `hide_empty` section key: config bool,
load-time placement checks, copied to the wire by `viewSectionToV1`. Panel
switch follows `mentioned_by_panel`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Config bools on `dataentryconfig.ViewSection` and
`EntityViewConfig`, validated in `validate.go`. The server sends one `hide` list
(`"type"`, `"id"`) per section and `hideMentionedByID` on the view response. The
SPA reads them through `hidesType`/`showsId` in EntityDetail and a `hideId` prop
on MentionedByPanel. Alternatives: hiding by default (rejected: changes every
existing project; IDs such as TKT-123 are how people refer to records); two wire
booleans (rejected: archplan flagged the flag count and a data clump).

**Files to modify:** internal/dataentryconfig/config.go, validate.go;
internal/apiwire/v1/responses.go; internal/dataentry/sections.go,
views_handler.go; frontend/src/api/views.ts, EntityDetail.vue,
MentionedByPanel.vue; docs.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Operator config only: booleans parsed by YAML,
placement checked at load; an invalid placement is a load error.

**Security-Sensitive Operations:** None. Presentation only: the ID and type stay
in the row link and the API, no ACL path reads the keys, and no endpoint or
existence signal is added. Docs say it is not access control.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria.

**Edge Cases:** Empty or blank title keeps the ID; omitted keys keep current
output; `nested` children rows honour `hide_id`.

**Negative Tests:** Keys on `properties`/`table`, on `source: entry`, in a side
panel, and `hide_type` on `nested` fail at load.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** An author may think hiding the ID conceals it; the docs state it is
presentation only. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md (via docs-project GUIDE-data-entry) and docs/data-entry/api-reference.md

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-Q7B99XW (minor, addressed), RR-BGC2ROA (minor,
deferred). The design note was written before the code; the design review ran
after implementation because the review skill could not run in the implementing
session. No critical or significant findings.
