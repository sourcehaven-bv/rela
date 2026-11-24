---
id: PLAN-1YE47F9
type: planning-checklist
title: 'Planning: Nested generated sidebar groups with per-entry counts'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Understanding
- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

In: one child level on `items_from` and `status:` counts on generated entries
and children. Out: deeper nesting, per-rule conditions. Acceptance: children
grouped in list order and capped per parent; hidden rows neither shown nor
counted; zero count omitted; reads fixed per group; sidebar payload carries no
titles.

## Research
- [x] ~~For larger features: run `/research`~~ (N/A: extends items_from with existing patterns)
- [x] ~~Searched for existing libraries~~ (N/A: internal feature)
- [x] Checked codebase for similar patterns or reusable code (navItemNeighbors, configuredListRows, nav status)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal feature)
- [x] Reviewed relevant rela concepts for prior art (data-entry-ui, authorization)

## Approach
- [x] Technical approach chosen and documented (.ignored/design-TKT-MQSK037.md, shared notes)
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (per-rule conditions rejected: the counted list's condition decides)
- [x] Dependencies identified (store.RelationQuery, configuredListRows, rela-components NavItem.children)

## Security Considerations
- [x] Input sources identified (operator config only; no new request parameter)
- [x] Input validation approach defined (allowlist of lists, relations, tones; bounded limits)
- [x] Security-sensitive operations identified (row gate and redaction of child and counted rows)
- [x] Error handling doesn't leak sensitive information (a failing group is skipped and logged)

## Test Plan
- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented (no edges, hidden child at the limit)
- [x] Negative test cases defined (config validation table)
- [x] Integration test approach defined (handler tests through the real ACL; budget test)

## Risk Assessment
- [x] Technical risks assessed with mitigations (nil reach reads everything: skipped when empty)
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (m)

## Documentation Planning
- [x] User-facing docs identified (GUIDE-data-entry, GUIDE-acl-security)
- [x] Docs-checklist will be created when entering implementation
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] docs/data-entry.md - UI changes
- [x] ~~CLAUDE.md~~ (N/A: no new convention)
- [x] ~~README.md~~ (N/A: no project-level change)
- [x] ~~N/A~~ (N/A: there are user-facing docs)

## Design Review
- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan
