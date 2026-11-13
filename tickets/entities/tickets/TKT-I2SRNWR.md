---
id: TKT-I2SRNWR
type: ticket
title: LLM actions on collection screens (kanban, lists, piles)
kind: enhancement
priority: medium
effort: l
tags: needs-design
status: backlog
---

## Description

Extend declarative LLM actions (PR #2246, FEAT-QLU1C1B, TKT-4USH1HO) from one
entity to the rows on a screen: a kanban board or column, a list selection, or a
pile. One model call sees the rows and proposes changes per row. Depends on PR
#2246 landing.

A single card already works through the kanban overlay panel, which mounts
`EntityDetail` and its proposal dialog. What is missing is the screen as the
input.

### Use cases

1. Triage a column or selection: propose priority, assignee and labels for
each card in Inbox.
2. Plan across the board: which backlog cards fit this sprint, which
in-progress cards look stuck.
3. Summarize the board (answer only, nothing written). Optional for v1.

### Proposed design

- **Binding:** referenced from the surface, like list `toolbar_actions`. Add
the key to kanban, plus an optional column-header menu; allow list `actions:`
(selected rows) and `piles.actions`. The action has no `available_on`, so it
stays apart from entity-bound actions.
- **Input:** the server resolves the rows itself from the surface config and
the client's filters, under the caller's ACL, as `ExportRender` does. It never
accepts a list of ids from the client. Each row sends only declared `fields`,
with a per-row size cap; the total stays under `MaxLLMTotalItems`.
- **Output:** proposals keyed by row id. An id outside the rows sent is
rejected, as candidate ids are today. `set` and `relate` per row; a kanban
column move is `set` on the column property, checked against its enum.
- **Review and apply:** a batch dialog grouped by row, one tick per change,
applied through the normal PATCH and relation APIs. Keep the stale-value check
and the per-item failure report; the board changes live while the dialog is
open.

### Risks

- Prompt injection across rows: one card's text can steer proposals for
others. Per-change review limits it; v1 allows property outputs only.
- Rows are row-gated and redacted before they are sent (the gantt order).
Counts and truncation flags are computed after filtering.
- One call per action fits the existing synchronous request and timeout.

### Out of scope

- Running the per-entity action once per row (N calls). That belongs on
`jobs.Queue` with progress reporting.
- Proposals shown as hints on the cards themselves.
- `create` outputs.
