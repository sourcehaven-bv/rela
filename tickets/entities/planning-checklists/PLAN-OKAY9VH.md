---
id: PLAN-OKAY9VH
type: planning-checklist
title: 'Planning: Fix _views path in retired-id redirect docs'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: the `_views` path in the acl-security guide section on retired-id redirects
(source and generated file).

OUT: any code change. The code is correct.

**Acceptance Criteria:**

1. The guide names the route as `/api/v1/_views/<type>/<id>`. Check: grep for
`_views/<view>` in `docs-project/` and `docs/` returns nothing.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: one-word docs fix)
- [x] ~~Searched for existing libraries~~ (N/A: docs fix)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: docs fix)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: docs fix)

**Research Doc:** N/A

**Existing Solutions:**

`handleV1Views` (`internal/dataentry/views_handler.go`) parses
`/api/v1/_views/{entityType}/{entityId}`, and `viewMiss` builds the redirect
`Location` from the entity type. The data-entry guide already writes `{type}`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] ~~Alternatives considered~~ (N/A: the only option is to correct the text)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** replace `<view>` with `<type>` and run `just docs`.

**Files to modify:** `docs-project/entities/guides/GUIDE-acl-security.md`,
`docs/acl-security.md` (generated).

## Security Considerations

- [x] ~~Input sources identified~~ (N/A: docs-only change)
- [x] ~~Input validation approach defined~~ (N/A: docs-only change)
- [x] ~~Security-sensitive operations identified~~ (N/A: docs-only change)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: docs-only change)

**Input Sources & Validation:** N/A

**Security-Sensitive Operations:** N/A

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: docs-only change)
- [x] ~~Negative test cases defined~~ (N/A: docs-only change)
- [x] ~~Integration test approach defined~~ (N/A: docs-only change)

**Test Scenarios:** AC1 is checked by grep.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed~~ (N/A: docs-only change)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** none beyond the generated file drifting from its source; mitigated by
regenerating with `just docs`.

**Effort:** xs

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** the acl-security guide only.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one-word docs fix)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** none.
