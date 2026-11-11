---
id: TKT-V51KJDF
type: ticket
title: 'Direct messages 1/4: managed _dm type and participants-only read route'
kind: enhancement
priority: medium
effort: l
status: backlog
description: Managed _dm entity type with _member edges and a participants-only ACL read route (DEC-42MOZY2).
---

## Description

Add the rela-managed `_dm` entity type (id prefix `DM-`) and its managed
`_member` relation to the user entity type, following `_flow-run`
(`internal/metamodel/flowrun.go`, `internal/acl/managed.go`). Design:
DEC-42MOZY2, discussion: TKT-55RNWWD.

- rela adds the type and relation only when DMs are enabled. `schema.yaml`
and `acl.yaml` cannot name them; `"*"` does not cover them.
- A managed role for `system:realtime` is the only writer.
- Read route: a principal reads a `_dm` exactly when its user entity is the
target of one of the `_dm`'s `_member` edges. Compiled at load, lowered to a
store query, applied by the visibility wrappers. Leave room for a later operator
grant on the same route set.
- Same exclusions as `_flow-run`: no automations, validation, comments,
import, rename, convert, migrations. Not in menus or list pages.
- Id: hash of the sorted pair of user entity ids.

## Acceptance criteria

1. A participant reads the `_dm`; a third person, a role with `read: ["*"]`
and a project without acl.yaml get the uniform 404.
2. acl.yaml naming `_dm`, `_member` or `system:realtime` fails to load;
schema.yaml declaring `_dm` fails to load.
3. Only `system:realtime` can create, update or delete a `_dm`.
4. Deleting a participant's user entity removes the `_member` edge; the
`_dm` then reads as hidden to everyone.
5. Every store backend passes a storetest case for the read route.

Run `/design-review` during planning: this ticket carries the privacy guarantee.
