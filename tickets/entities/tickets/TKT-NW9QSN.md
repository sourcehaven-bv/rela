---
id: TKT-NW9QSN
type: ticket
title: MCP tools for running actions
kind: enhancement
priority: medium
effort: m
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

## Problem

Configured data-entry actions (`actions:` in data-entry.yaml, run through `POST
/api/v1/_action/{id}`) can only be started from the UI. An agent working through
the remote MCP (for example against Atlas) cannot run them, so generator actions
such as "Genereer meting" (AAL2 measurement), "Nieuwe VvT", the audit programme
and the control-execution overview need a human click.

## Goal

MCP tools to discover and run actions, with exactly the gating of the HTTP
route: `list_actions` and `run_action`.

## Scope

In scope:

- `list_actions`: without `id`, the actions not bound to an entity that the
caller may run (permission held); with `id` (`ID` or `ID@face`), the
entity-bound actions (`available_on`) the detail page would offer for that
entity. Each with id, label, description, confirm text, whether it is
request-scoped.
- `run_action`: action id, optional entity address, optional request body
(only for actions with a `request:` block), and `confirm: true` required for
actions that declare `confirm:`. Result: message, message_type, redirect, or for
a rich response status/content_type/body; script errors as a tool error with the
correlation id.
- The HTTP handler and the MCP tool share one operation (permission check,
entity resolution under the visibility reader, detail-action gate, capabilities,
provisioning, config-reload refusal), as was done for comments (TKT-QIQO1W).
- Remote MCP only, running as the signed-in principal. Not registered on
stdio `rela mcp`, which runs as the system principal and already has `lua_run`.

Out of scope:

- Built-in entity actions (`/_actions/clone`).
- Webhooks and commands (`commands:`).
- Period parameters for specific actions: that is the action script's own
contract (`request:` block), in the consumer project.
