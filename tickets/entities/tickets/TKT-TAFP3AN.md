---
id: TKT-TAFP3AN
type: ticket
title: Resolved kanban board endpoint
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

`KanbanView.vue` fetches `/_config` plus the whole list and builds the board:
columns from the enum or `columns_from`, swimlanes, related card values
(`useRelatedCardValues.ts`), summaries and create targets. Add `GET
/_kanbans/{name}` that returns the resolved board with pagination per column,
batched per page, with a budget test. The SPA renders it and drops the grouping
code.

Part of FEAT-QRI3CFR (step 2.1).
