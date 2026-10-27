---
id: TKT-RBWCSW
type: ticket
title: Write-time roll-up reads (automations, validations, transition guards)
kind: enhancement
priority: low
effort: l
status: backlog
---

## Description

Follow-up of TKT-6I2OPG (user decision option C). Roll-ups are read surfaces
only; any predicate naming one fails to load. Options: (A) compute roll-ups for
the written entity with the writer's gate before automation
`condition:`/`validate:` and validation rules run; (B) a `count_related()`-style
predicate evaluated through relresolve. Note that transition guards cannot read
entity properties at all today (`internal/statemachine/predicate.go` exposes
only id, type and value plus has_relation/count_relations), so guards need that
first.
