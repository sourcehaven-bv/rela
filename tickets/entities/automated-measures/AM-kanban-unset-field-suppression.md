---
id: AM-kanban-unset-field-suppression
type: automated-measure
title: 'Test pin: card fields with unset values render no row (no dangling label / empty badge)'
description: The "KanbanView card fields with unset values" Vitest suite renders a card with a configured-but-unset enum field and asserts the field row is absent, plus the positive case that a set value still renders. Pins the unset-value presentation contract for config-driven card surfaces (BUG-K4NBF2).
kind: test
location: frontend/src/views/KanbanView.test.ts
status: active
---
