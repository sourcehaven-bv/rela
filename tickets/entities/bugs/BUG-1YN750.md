---
id: BUG-1YN750
type: bug
title: Family delete authorizes one face and deletes all faces
description: DeleteEntity authorizes delete on the face anyFaceOf returns, then deletes every face, so a draft-only deleter can remove the published face.
priority: critical
effort: m
why1: DeleteEntity authorized OpDelete for the single row anyFaceOf returned (the first face in id order) and then called store.DeleteEntity, which removes every face of the family.
why2: The BUG-HC6I2T fix replaced GetEntity with anyFaceOf to find the entity type; anyFaceOf answers a type question, but its row also supplied the face for the ACL subject.
why3: Face-qualified write grants arrived after DeleteEntity was written for one row per id; nothing re-derived which rows a bare-id delete removes when faces were added.
why4: No test paired a per-face delete grant with a multi-face entity; delete tests used NopACL or type-wide grants, so one-face and all-face authorization were indistinguishable.
why5: Faces were bolted onto APIs built for one record per id (DEC-NPZICR), so every family-wide operation must be checked by hand for which faces it touches; there is no structural rule that authorization covers exactly the rows written.
prevention: Authorize every face the family delete removes, re-checked inside the Tx; regression test TestFamilyDelete_* runs a per-face delete grant against memstore, fsstore, sqlite and postgres. DEC-NPZICR stages the structural fix (typed face addresses).
status: done
---

## Problem

`Manager.DeleteEntity(id)` deletes the whole family of an entity, meaning every
face. It authorizes only one face: `anyFaceOf` returns an arbitrary face, and
the `OpDelete` subject carries that face
(`internal/entitymanager/manager.go:1437-1450`). A principal allowed to delete
`policy@draft` but not `policy@published` can remove the published face whenever
`anyFaceOf` returns the draft. Only one face gets a version capture and an audit
record.

Reached through every bare-id delete: CLI `delete`, MCP `delete_entity`, Lua
delete.

Verified by reading the code. Found by the face-awareness inventory (BUG-CTUW2N,
`.ignored/face-awareness-inventory.md`).

## Expected

A family delete authorizes `delete` on every face it removes, and fails closed
if any face is denied. Each removed face is version-captured and audited.
