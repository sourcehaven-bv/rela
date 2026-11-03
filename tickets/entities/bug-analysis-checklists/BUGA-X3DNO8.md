---
id: BUGA-X3DNO8
type: bug-analysis-checklist
title: 'Analysis: rela.md.parse drops the closing line of multi-line HTML blocks'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Notes:** Reproduced with `rela.md.render(rela.md.parse(s))` on any HTML
comment that spans more than one line; independent of backend or platform. Only
`ast.HTMLBlock` carries a separate `ClosureLine` in goldmark; fenced code blocks
go through `extractCodeBlockContent` and keep their fence.
