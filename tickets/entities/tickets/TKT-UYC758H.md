---
id: TKT-UYC758H
type: ticket
title: Remove the jump bar from the entity detail page
kind: chore
priority: low
effort: xs
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

The entity detail page shows a jump bar: a row of buttons above the sections,
one per section heading, that scrolls to that section. Remove it.

## Scope

In scope: the `nav.jump-bar` template block, its `.jump-bar`/`.jump-link` CSS,
and the `scrollToSection` helper in
`frontend/src/components/entity/EntityDetail.vue`.

Not in scope: section ids, section rendering, or other navigation.

## Acceptance criteria

- The entity detail page renders no jump bar for any view, including views with several sections.
- No dead code remains (`scrollToSection`, jump-bar CSS).
- Typecheck, lint and EntityDetail tests pass.
