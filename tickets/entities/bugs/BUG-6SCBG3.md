---
id: BUG-6SCBG3
type: bug
title: DeleteEntity authorizes one face and deletes every face
description: Manager.DeleteEntity on a bare id authorizes against the first face anyFaceOf returns and then deletes the whole family, so a face-scoped delete grant can delete a face it may not read.
priority: high
status: backlog
---

## Description

`Manager.DeleteEntity(bare id)` authorizes against whichever face `anyFaceOf`
returns first (`internal/entitymanager/manager.go`, `core.go`) and then deletes
every face. A principal with `delete: [policy@concept]` may therefore delete a
`published` face it cannot read, depending on store order.

Reachable through web DELETE, Lua `rela.delete_entity`, and (since BUG-6XTX0G)
MCP `delete_entity`. Found in the BUG-6XTX0G security review.

## Fix

Authorize every face in the family before deleting it.
