---
id: TKT-WQ925LL
type: ticket
title: 'LLM actions: follow-ups (auto-apply, create outputs, batch runs)'
kind: enhancement
priority: low
status: backlog
---

## Description

Follow-ups to TKT-4USH1HO (the `llm:` block on data-entry actions,
propose-then-accept).

- **Auto-apply.** An opt-in mode that applies the proposal without the dialog, run on `jobs.Queue` (an AI call is an external side effect).
- **Create outputs.** Let the model propose a new entity (for example a sub-task) as well as properties and relations.
- **Flow steps.** Run an llm task as a step in a flow.
- **List and batch runs.** Run the action from a list toolbar over selected rows, as background jobs.
- **Server-level provider config.** Allow the AI provider to be set per server, not only in `.rela/ai.yaml`.
- **Candidate condition pushdown.** Lower the `candidates` condition into the store query when `queryplan.LowerScope` lowers it exactly, instead of matching each page in Go.
- **Lower `max` by existing relations** (RR-ZXPYDQJ): narrow the proposal schema by the relations the entity already has.
