---
id: TKT-EC7F65
type: ticket
title: Lua history API for entity versions
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Expose entity history to Lua scripts: list an entity's versions and read the
entity as it was at a given version. A sync connector needs this to compute a
3-way merge against the base, which is the version at the last sync
(FEAT-XYQMUB).

## Scope

- Backed by `store.HistoryReader` (postgres and sqlite).
- On a backend without versioning, the binding raises a clear "unsupported on
this backend" error. It never returns empty history, because an empty base would
make every field look changed on both sides.
- Reads go through the same ACL and field redaction as other Lua reads.
- Out: relation history, restore, purge.
