---
id: TKT-EY4W0P
type: ticket
title: 'notify automation action: push to related users, skipping the actor'
kind: enhancement
priority: medium
effort: m
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

## Description

Add a `notify` automation action that puts the triggering entity on a declared
pile of the people related to it, with the acting user skipped. Common case:
when someone completes a task of a project, the project owner is notified.

```yaml
automations:
  - name: notify-project-owner
    on:
      entity: [task]
      updated: true
      condition: "entity.status == 'done' and old.status ~= 'done'"
    do:
      - notify:
          pile: notifications            # declared pile key
          to: {via: belongs-to, property: owner}
          reason: completed
```

## Design

- `to:` forms: a property of the triggering entity (`{property: assignee}`), or a traversal then a property (`{via: belongs-to, property: owner}`). `via` uses the same path syntax as `related(...)`, so multi-hop paths work. A list property or a traversal reaching several entities fans out to every recipient. Duplicate recipients collapse.
- The acting user is always skipped: notifications are for what others do.
- The pile must be a declared pile with `read_state: true`; the reason must be one of its `reasons:`. Both are checked at load.
- The push records the actor and reason on the item.
- Best-effort like `add_to_pile`: a failed push is logged and never fails the write.
- Recipient traversal reads run with an allow-all reader under the system principal (the recipients come from config, not from the actor's view). The recipient's own gate still applies when they read the pile, so a hidden entity never shows.
- Bulk edits push one item per entity; `on_repeat: bump` keeps repeats from duplicating.
- Lua stays available for cases `to:` cannot express; extend `rela.piles` with the same actor/reason fields.

## Depends on

The declared-piles ticket and the read-state ticket.
