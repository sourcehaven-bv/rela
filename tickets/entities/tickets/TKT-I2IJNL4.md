---
id: TKT-I2IJNL4
type: ticket
title: 'Flows: let outside_edits cover the body, relations and delete'
kind: enhancement
priority: low
status: backlog
---

## Description

TKT-XI7IDBS added `outside_edits:` to flows.yaml. It narrows the writable
properties of a subject while a run holds it. It does not narrow the entity
body, relations or deletes, and it does not cover surfaces that do not check
field rules (CLI, stdio MCP, operator `_actions`, automations,
`rela.bypass_acl`, CalDAV). Renames and conversions are already covered: rela
refuses to rename or convert a subject with an open run, on every surface.

Decide whether a flow should also be able to lock the body, its relations and
deletion of the subject while a run is open, and on which surfaces. The
data-entry PATCH and the field-gated manager are the natural places; the
relation and delete write paths would need their own check.
