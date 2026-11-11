---
id: BUG-FCQBONH
type: bug
title: 'Copy with relations: replace keeps target edges when the source face has none of that type'
description: A replace copy only replaces relation types that have at least one source edge, so removing the last edge on a draft face is never carried to the promoted face.
priority: high
effort: s
why1: applyCopyEdges ran the replace only for relation types in plan.edges, and planCopyEdges only marked target edges removable for types with a creatable source edge.
why2: Both loops were keyed on the source face's edges, so a type with zero source edges was never visited.
why3: The copy kernel treated replace as 'delete then add for each copied type' instead of 'make the target set equal the source set', and the comment recorded that as intended.
prevention: TestCopyReplace_EmptySourceEmptiesTarget covers the empty source set, with and without the create right. Replace tests now include the empty-set case alongside the one-edge case.
status: backlog
---

## Description

A copy definition with `relations: { <type>: replace }` does not remove the
target face's edges of that type when the source face has no edges of that type.

Example: an entity has a `draft` and a `published` face. The user removes every
`implements` edge from the `draft` face and runs the copy that promotes `draft`
to `published`, with `implements: replace`. The `published` face keeps all its
old `implements` edges. Removing all but one edge works, because then one edge
is copied and the replace runs.

## Cause

`planCopyEdges` (`internal/entitymanager/copy.go`) only marks a target edge
removable when its type is in `creatable`, and `creatable` is filled from the
source face's edges. `applyCopyEdges` (`internal/entitymanager/copy_apply.go`)
only runs the replace for types present in `plan.edges`. A type with zero source
edges therefore never reaches the replace step. The comment "Only a type that
copies at least one edge replaces anything" documents this, but it contradicts
what `replace` means: the target ends with the source's edges of that type,
including none.

## Expected

After a copy with `replace`, the target face holds exactly the source face's
edges of that type that the principal could create, minus edges the principal
may not remove. An empty source set empties the target set (within the removal
gate).

## Fix direction

- Decide the removal gate per `replace` type named in the definition, not per type seen on the source.
- Run the replace for every `replace` type with removable target edges, also when no edge of that type is copied.
- Keep the existing rules: unreadable peers stay, a type the principal may not remove stays.
- Add a test in `copy_keep_test.go` or a new `copy_replace_empty_test.go`: target face with edges, source face with none, copy with `replace`, expect the target edges gone.
