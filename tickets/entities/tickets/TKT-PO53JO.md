---
id: TKT-PO53JO
type: ticket
title: Lua code sharing between scripts (shared modules via require)
kind: enhancement
priority: medium
effort: m
started: "2026-10-13"
completed: "2026-10-14"
status: done
---

## Description

Lua scripts in a rela project (actions, documents, scheduled jobs, automations,
flows, migrations, mail transports, export renderers) cannot share code. Common
helpers (formatting, entity lookups, date handling, mail layout) are copy-pasted
between scripts and drift.

Decide and implement how a project shares Lua code between scripts: where shared
modules live, how a script loads them, and how that stays inside the sandbox
(path containment, no filesystem escape, works on every storage backend,
respects read vs. write runtimes).

The planning phase records options, the recommendation, scope and acceptance
criteria.
