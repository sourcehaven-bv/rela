---
id: TKT-XI7IDBS
type: ticket
title: 'Flows M3: policy for edits outside a flow'
kind: enhancement
priority: medium
effort: l
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

## Description

Policy for edits outside a flow while a run is active.

```yaml
outside_edits:
  default: allow
  properties:
    ernst: deny
    risico_betrokkenen: { permission: privacy.officer }
  states:
    melden-ap: { default: deny }
```

- Compiles at load time into a new allowlist dimension of the ACL ceiling
(`effective = grants ∩ flow ceiling`); no runtime deny
(`.claude/rules/acl-ceiling.md`).
- With several active runs on one entity, their allowlists intersect.
- Writes by the flow's own steps are governed by the step's `writes`.

## Acceptance

- Design confirms "run in state S" lowers to a `related()` predicate on
every backend.
- Tests cover allow, deny, permission, per-state override and two
overlapping runs.
