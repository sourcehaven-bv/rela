---
id: TKT-J3RDJ4
type: ticket
title: Webhook principals get principal_type webhook; acl map --webhook with --expect CI gate
kind: enhancement
priority: medium
effort: m
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

## Description

A declarative webhook (`POST /hooks/<id>`) writes as `system:webhook:<id>`. That
principal carries no `principal_type`, so no `client_baselines:` ceiling can
match it. It also holds the built-in `everyone` role, so a webhook can do
everything `everyone` can.

1. Stamp the hook principal with a verified `principal_type: webhook`, so an
operator can cap every webhook with one baseline (`applies_to: [webhook]`). A
deployment without such a baseline behaves as before. Foreground automations and
Lua inside the webhook save run under the same principal and are capped too;
background `run_as` actions are not.
2. Give operators a CI-usable check of what a webhook may do:
`rela acl map --webhook <id>` applies the runtime ceiling without the operator
naming the principal type, and reports entity verbs, permissions and relation
writes. `--expect <file>` fails with exit 1 and a diff when the effective access
differs. `rela acl audit` flags a configured webhook that no baseline caps.

Consumer: Atlas caps its Icinga hooks to read/create/update `incident`.
