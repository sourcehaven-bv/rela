---
id: BUG-1YN750
type: bug
title: Family delete authorizes one face and deletes all faces
description: DeleteEntity authorizes delete on the face anyFaceOf returns, then deletes every face, so a draft-only deleter can remove the published face.
priority: critical
effort: m
status: backlog
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
