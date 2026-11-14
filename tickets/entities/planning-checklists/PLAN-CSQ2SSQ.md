---
id: PLAN-CSQ2SSQ
type: planning-checklist
title: 'Planning: Data entry guide: total card min and max property types'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the `min`/`max` bullet in "Total cards" of GUIDE-data-entry.md
and its generated copy. Out: code changes; other data entry guide issues
(#2232).

**Acceptance Criteria:**
1. The guide names `integer`, `decimal`, `date` and `datetime` as the property types `min` and `max` accept. Checked by reading `summaryPropertyTypes` in `internal/dataentryconfig/summary.go` against the new text.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: one-sentence docs fix)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: docs fix)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: docs fix)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** The `sum`/`avg` bullet in the same section already names
its types; the new wording follows it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] ~~Dependencies identified (packages, APIs, types)~~ (N/A: docs only)

**Technical Approach:** Edit the bullet in docs-project and regenerate docs with
`just docs`. Widening the code to accept `string` and `enum` was rejected: the
type list is shared with kanban `column_summary` and is the intended design of
TKT-N7P2RUP.

**Files to modify:** docs-project/entities/guides/GUIDE-data-entry.md,
docs/data-entry.md (generated).

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: docs only)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: docs only)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: docs only)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: docs only)

**Input Sources & Validation:** N/A

**Security-Sensitive Operations:** N/A

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: docs only)
- [x] ~~Negative test cases defined (invalid input, error conditions)~~ (N/A: docs only)
- [x] ~~Integration test approach defined (not just unit tests)~~ (N/A: docs only)

**Test Scenarios:** Manual comparison of the text with `summaryPropertyTypes`;
`just docs` regenerates with no diff beyond the bullet.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: docs only)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** None beyond wording; effort xs.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/data-entry.md only.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one-sentence docs fix)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review for a docs fix)

**Design Review Findings:** none
