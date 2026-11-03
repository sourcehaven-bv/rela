---
id: TKT-26UHHKL
type: ticket
title: 'Dashboard: one card per row on phones'
kind: enhancement
priority: medium
effort: s
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

## Description

On phones (viewport <= 480px) the dashboard grid has two tracks. Only breakdown
and table cards span the full row, so count cards sit two per row. Jeroen
decided that on phones every card takes one row, whatever its `width` setting.

## Approach

In `frontend/src/views/DashboardView.vue`, the <= 480px rule becomes
`grid-template-columns: minmax(0, 1fr)`. The two `:has(...)` full-row rules are
removed. The card-width comment says "one fixed track on phones". Update the
phone sentence in the `width` docs in `docs/data-entry.md`, and assert one track
and stacked cards at 375px in `e2e/tests/dashboard-card-width.spec.ts`.
