---
id: TKT-EO2GCM
type: ticket
title: 'Declared piles: operator-defined, locked piles in schema.yaml'
kind: enhancement
priority: medium
effort: m
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

## Description

Let the operator declare piles in `schema.yaml`. A declared pile exists for
every user, is found by a stable key instead of its name, and is locked: the
user cannot rename or delete it. Declared piles are the base for favorites (star
toggle) and notifications.

```yaml
# schema.yaml
piles:
  favorites:
    name: Favorites
    icon: star
  notifications:
    name: Notifications
    icon: bell
    on_repeat: bump        # keep (default) | bump
```

The declaration lives in `schema.yaml`, not `data-entry.yaml`, because the piles
service enforces its behaviour for every push: automations, Lua and MCP, not
only the web app.

## Scope

- `piles.Pile` gains `Key`. Both backends (kvpiles, pgpiles) store it; `pilestest.RunAll` covers it.
- The service creates a declared pile on first use for an owner. It does not count toward `MaxPiles` or `MaxForeignPiles`.
- Rename, re-icon and delete of a declared pile are refused. The name and icon come from config.
- Lookup by key (`ByKey`); pushes and Lua/MCP can address a declared pile by key.
- `on_repeat: bump` moves an item already on the pile to the top instead of keeping its place.
- A user pile with the same name as a declared pile can coexist; the key, not the name, identifies the declared one.
- Load-time validation of the `piles:` block (known keys, icon names, unique names).
- The SPA does not list declared piles in the "Piles" group. They appear only where the operator places them (see the navigation ticket).
- Removing a declaration from config: decide whether its stored piles are dropped or become ordinary user piles. Recommendation: become ordinary user piles, so no data is lost.

## Out of scope

Read state, expiry and the `notify` action have their own tickets.
