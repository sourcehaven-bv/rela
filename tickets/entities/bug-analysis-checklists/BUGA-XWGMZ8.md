---
id: BUGA-XWGMZ8
type: bug-analysis-checklist
title: 'Analysis: Section create menu has no background since the move to rela-components'
started: "2026-10-06"
completed: "2026-10-06"
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

## Notes

- Reproduced against a production build of develop (b82c61225): the
`EntityDetail` chunk ships `.create-menu{...background:var(--bg-primary)...}`,
and no emitted stylesheet defines `--bg-primary`, `--bg-secondary` or
`--text-primary`.
- Steps: open an entity whose detail page has a section with a create action
that offers more than one type, click "+ New". The menu has no background and
items show no hover highlight.
- Fix: use `--rl-color-bg-raised`, `--rl-color-bg-hover` and `--rl-color-text`,
as the status menu does. Same sweep in ProjectSwitcher, WorldSwitcher,
InlineCreateFormModal and RelationCards.
- Regression test: source guard over every `var()` without a fallback.
