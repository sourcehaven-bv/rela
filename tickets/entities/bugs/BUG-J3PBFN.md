---
id: BUG-J3PBFN
type: bug
title: CLI, MCP, Lua, automation and importer writes read the zero-face row
description: Delete, create_relation and import outside the data-entry app look up or create zero-face rows on faced types.
priority: medium
effort: m
why1: Each write path outside data-entry resolved its id with GetEntity or GetEntityState(id, ""), which reads the zero-face row a faced type never stores, and wrote relations with the default tail.
why2: These paths predate faces; data-entry was migrated to addresses first and the others kept their bare-id reads because nothing failed on the faceless fixtures they were tested with.
why3: Every test fixture outside data-entry and entitymanager seeded faceless types only, so a faced entity never reached CLI, MCP, Lua, the cascade host or the importer in a test.
why4: The zero-face allowlist pinned these reads as existing debt but nothing tied the debt to a per-surface faced test; the cascade host also built its own delete instead of calling the manager's family path.
why5: Face-awareness was rolled out surface by surface without a shared rule that every write surface must accept ID@face and be tested with a faced fixture.
prevention: Every write surface outside data-entry has a faced-fixture test for ID and ID@face (AM-writes-outside-dataentry-address-faces); the zero-face allowlist shrank by the reads this fix removed and may only shrink further.
status: done
---

## Problem

Reported by the face-awareness inventory, not yet verified. Write paths outside
the data-entry app read the zero-face row:

- Entity and relation delete over CLI, MCP and Lua (`cli/delete.go:26`, `mcp/tools_entity.go:421`, `lua/runtime.go:1976,2042`).
- Automation `create_relation` loses faced targets and the tail (`autocascade/runner.go:249-263`, `cascadehost.go:115`).
- The importer writes bare rows on faced types (`importer.go:80,450-458`).

## Expected

These paths accept `ID@face` and never create or look up a zero-face row for a
type that declares faces.

## Also in scope (reported by the BUG-1YN750 fix, #1708)

- `internal/entitymanager/cascadehost.go:182`: an automation-driven delete is face-blind; it writes one audit record and no per-face version capture.
- CLI `delete` and MCP `delete_entity` answer "entity not found" for a faced entity.

## Remaining zero-face reads in entitymanager (from BUG-58BL9I review)

- `findExistingRelationTarget` in `internal/entitymanager/core.go` and `internal/entitymanager/apply.go` read at the zero face and miss faced entities.
- `DeleteEntityFace` reads the face outside the transaction.
- `relation_grants` evaluation ignores faces.

## Stage 2 scope (moved from BUG-BZQQDP)

- Unlink over CLI, MCP and Lua, and a zero `RelationOptions{}`, call the
default-tail `DeleteRelation`, so a content-scoped edge on a face cannot be
removed through them. The data-entry relation routes already address the tail.
Stage 2 makes the tail part of every relation call (`RelationKey` on
`Get/Create/Update/DeleteRelation`, TKT-KQXVF7 PR 7), which fixes these callers.

## Resolution (TKT-KQXVF7 PR 3)

- A bare id is the family and `ID@face` is one face on every surface: CLI
delete, MCP `delete_entity`, Lua `delete_entity` and admin `delete_entity`. A
face delete always removes the edges tailed at that face; `cascade` guards the
family delete only.
- Unlink is included on the current store API: CLI `unlink ID@face`, MCP
`delete_relation` with `from: ID@face`, and Lua `delete_relation` with
`opts.face` call `DeleteRelationState`. It did not need the relation flip. This
also covers BUG-YVU8CP's symptom.
- `findExistingRelationTarget` and `apply.go` read families by header;
automation `create_relation` and `if_exists: replace` keep the trigger's tail.
The cascade-host delete goes through `Manager.DeleteEntity`, so it writes one
audit record and one version per face.
- `DeleteEntityFace` re-reads the row inside the transaction and
re-authorizes if its type changed.
- Ruling D4: a zero-tailed edge from a faced source is authorized on every
stored face; a named tail is authorized at that face; `relation_grants` does not
satisfy an edge whose tail face the principal cannot update.
- The importer accepts `ID@face` rows and tails, and refuses a bare id on a
faced type, an undeclared face, and a tail on an identity relation.
- pgstore `DeleteEntityState` takes the family advisory lock (A2). The old
`FOR UPDATE` was on the bare row, which a faced type does not store, so it
locked nothing; the race was serialized only by the create's `FOR SHARE` on
sibling rows. `TestDeleteEntityState_RacingFaceCreateKeepsAttachments` pins the
lock.
- The zero-face allowlist lost 13 reads in 6 files.
