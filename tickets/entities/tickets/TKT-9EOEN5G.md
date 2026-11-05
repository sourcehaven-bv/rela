---
id: TKT-9EOEN5G
type: ticket
title: Resolved form endpoint
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Forms are assembled in the SPA from config, schema, user defaults and templates.
Add `GET /_forms/{name}` that returns steps, fields, widgets, options, defaults,
create targets and affordances in one payload, and render `DynamicForm` from it.

Part of FEAT-QRI3CFR (step 3.1).
