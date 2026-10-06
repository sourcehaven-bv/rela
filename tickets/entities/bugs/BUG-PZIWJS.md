---
id: BUG-PZIWJS
type: bug
title: Section create menu has no background since the move to rela-components
description: On an entity detail page, a section whose create offers several types shows a + New dropdown. The dropdown has no background, so its items float over the table below, and hovering an item shows no highlight. SectionCreateButton.vue styles the menu with var(--bg-primary), var(--bg-secondary) and var(--text-primary). None of these custom properties is defined, so the declarations are invalid at computed-value time and fall back to transparent. The same class of undefined names appears in ProjectSwitcher, WorldSwitcher, InlineCreateFormModal, RelationCards and RelationPicker.
priority: low
effort: xs
why1: 'The menu sets background: var(--bg-primary) and its hover sets var(--bg-secondary). Neither custom property is defined, so both declarations are invalid at computed-value time and resolve to transparent.'
why2: The names come from a different design system. They were never defined in this app, not before or after the move to rela-components (#1731); the real tokens were --card-bg and --hover-bg then and are --rl-color-bg-raised and --rl-color-bg-hover now.
why3: CSS accepts any var() name silently. The build, ESLint, vue-tsc and the unit tests all pass with an undefined custom property, and Vitest does not apply scoped SFC styles.
why4: No e2e test opens this menu, and the defect is only visible with a populated table underneath it. Review saw a plausible token name and had no tool that checks it against the defined set.
why5: Nothing in the toolchain ties a var() use to a definition. Six components carried the same class of defect (SectionCreateButton, ProjectSwitcher, WorldSwitcher, InlineCreateFormModal, RelationCards, RelationPicker).
prevention: Source guard frontend/src/styles/cssCustomProperties.test.ts fails when a .vue <style> block or .css file uses var(--x) without a fallback and --x is not defined in the SPA or the rela-components library.
started: "2026-10-06"
completed: "2026-10-06"
status: done
---
