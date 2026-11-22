---
id: TKT-9AYQAYK
type: ticket
title: 'Flows: step form from a data-entry form instead of writes:'
kind: enhancement
priority: medium
effort: m
started: "2026-11-19"
completed: "2026-11-22"
status: done
---

## Description

A flow state's step form is built from `writes:`, a plain list of subject
property names. Each field gets its type's default widget, the schema label and
the schema description. There is no way to set a widget, placeholder, hint,
span, default, `readonly`, `visible_when` or `required_when`, and no relation
fields. `writes:` is a poor way to configure a form.

Replace `writes:` with a reference to a named form in `data-entry.yaml`:

```yaml
collect:
  waiting_for: [hr]
  form: collect_documents
```

The form's `entity_type` must be the flow's subject type. The SPA renders the
step form from that form. Remove `writes:` outright; nobody uses it yet, so no
migration or deprecation period is needed.

## What `writes:` feeds today

The form's editable fields must take over each use:

- `inputNames` (`internal/appbuild/flowsteps.go`): a step that sends a
property outside the list answers 422, over HTTP and MCP (`complete_flow_step`).
- `flowLockedProperties` (`internal/appbuild/triggeracl.go`): the written
properties are locked against outside edits while a run is open.
- `waiting.writes` in the run and task payload (`internal/flowengine/person.go`,
`frontend/src/api/flows.ts`, MCP `show_flow_run`).

## Decisions

- A form with `relations:`, `steps:` or `body: true` is refused for a step:
a step writes properties only.
- `required_when` / `visible_when` are evaluated in the SPA only, as on a
normal form save; Go treats them as opaque.
- The form check runs where the engine is built and in `rela flow validate`,
next to the `action:` check, not in `flowcfg`.
- The run payload carries `waiting.form` and `waiting.fields` (the accepted
properties), so MCP agents see what a step takes.

## Acceptance

- `writes:` is gone from `flowcfg`, the docs and the wire types.
- The step form renders the named form's widgets and conditions (E2E).
- The step input check and the outside-edit lock use the form's editable
fields (tests).
