---
id: PLAN-KK2SE
type: planning-checklist
title: 'Planning: Resolve entity-ID references to titled links in Lua markdown output'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented with specific test scenarios

See full plan history. 20 ACs covered by table-driven tests; 11 review-responses
all addressed. Implementation completed in commit on this branch.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated (m)

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — `docs/lua-scripting.md` gains
`rela.md.resolve_refs` and `rela.md.entity_refs` subsections.
- [x] ~~CLI help text~~ (N/A: no CLI commands changed)
- [x] ~~CLAUDE.md~~ (N/A: new helpers on an existing surface, no new patterns)
- [x] ~~README.md~~ (N/A: no project-level changes)
- [x] ~~API docs~~ (N/A: covered by `lua-scripting.md`)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| RR | Severity | Status |
| -- | -- | -- |
| RR-UT6HS | significant | addressed |
| RR-IBJUF | significant | addressed |
| RR-J3IIA | significant | addressed |
| RR-FA0JG | significant | addressed |
| RR-W7S4F | minor | addressed |
| RR-ZPSR3 | minor | addressed |
| RR-OF26C | minor | addressed |
| RR-LK64P | minor | addressed |
| RR-P77NK | minor | addressed |
| RR-9XQGM | minor | addressed |
| RR-TFYL6 | nit | addressed |
