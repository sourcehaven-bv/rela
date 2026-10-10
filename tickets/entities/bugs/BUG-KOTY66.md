---
id: BUG-KOTY66
type: bug
title: Repair identity edges that earlier face moves put on a named face
description: Before BUG-QEC1XJ every migrate_face and adopt-face move re-tailed identity-scoped edges onto the destination face. Those edges cannot be updated through the manager and are removed when that face is deleted. Existing projects need an analyzer check and a repair that returns them to the zero tail.
priority: medium
effort: s
status: backlog
---

## Description

`migrate_face` (since 2026-09-10) and `rela migrate adopt-face` re-created every
zero-tail edge of a moved row on the destination face, including identity-scoped
edges. BUG-QEC1XJ stops new moves from doing this, but data already migrated
keeps the misplaced edges:

- `UpdateRelation` refuses them (`ErrFaceNotDeclared`).
- `DeleteEntityFace` on that face cascades them away, so discarding a draft removes the entity's owner.
- Readers that cannot read that face lose the edge.

`rename_face` now returns such an edge to the zero tail, but only for rows it
renames.

## Fix

Add an `analyze` check for identity-scoped edges on a named tail, and a repair
(an `adopt-face` option or a dedicated command) that moves them to the zero tail
with the same clash detection as `uncarriedEdges`.
