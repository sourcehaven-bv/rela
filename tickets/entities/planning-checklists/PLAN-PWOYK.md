---
id: PLAN-PWOYK
type: planning-checklist
title: 'Planning: Restructure rela.md AST: preserve inline structure (text → inlines)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented with specific test scenarios

Plan body retained in entity history. Summary:

- Goldmark-faithful Lua AST: blockquote/list-item gain `children`,
inline-bearing leaves carry `inlines`, table cells become inlines.
- Two flatteners: `renderInlines` (preserves syntax) and
`flattenInlines` (legacy policy).
- `resolve_refs` rewritten in this PR to walk the inline tree.
- 11 inline kinds; image alt as `alt_inlines`; soft/hard break as
synthetic inlines after Text.
- Inline constructors exposed: text, code_span, link_inline, raw_html.
- 20 ACs, corpus-based round-trip property test, benchmark gate ≤ 2×.

## Research

- [x] All four research items completed.

## Approach

- [x] All four approach items completed.

## Security Considerations

- [x] All four items completed.

## Test Plan

- [x] All four items completed.

## Risk Assessment

- [x] All three items completed.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — `GUIDE-lua-scripting` source
- [x] ~~CLI help text~~ (N/A: no CLI changes)
- [x] ~~CLAUDE.md~~ (N/A: no new package patterns)
- [x] ~~README.md~~ (N/A: no project-level changes)
- [x] ~~API docs~~ (N/A: covered by `lua-scripting.md`)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 14 RRs all addressed. See entity history.
