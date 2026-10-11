---
id: BUG-6OZBP9
type: bug
title: Data-migration face move and delete leave comment threads behind
description: Data-migration steps delete faces and entities on the raw store, below the entitymanager's AliasRewriter hook, so comment threads for the deleted address survive and a recreated face or entity inherits them.
priority: low
why1: 'The face move (applyFaceMove: rename_face, migrate_face, adopt-face) deletes the source face with store DeleteFace, and drop_entities (also the GC sweep) deletes with DeleteFamily. Neither notifies the comment service, so threads keyed on the old address survive.'
why2: 'Migration writes bypass the entitymanager on purpose (a sanctioned raw-store exception: no automations, no ACL), and comment upkeep is attached only to the entitymanager''s AliasRewriter hook.'
why3: The comment service uses AliasRewriter rather than store.EntityObserver because stores discard observer errors. That keeps comment upkeep correct on the entitymanager path but puts it on exactly one write path, so every raw-store writer has to wire it separately. None of the migration writers did.
why4: The raw-store exception list in CLAUDE.md names the obligations such a writer carries (operator trust, audit, attribution, synchronous version capture). Version capture was wired into every migration delete because it is on that list. Id-keyed side stores such as comments are not on it.
why5: 'Systemic: side stores keyed by entity address are kept in step by an entitymanager hook, and there is no shared notification for raw-store writers. Each new raw writer or new side store has to remember the other, and the gap only shows when a thread reappears on a recreated face.'
prevention: 'P1: route every migration relocation through the two existing choke points (applyMoves, dropEntitiesStep) and have both call one consumer-side CommentThreads dependency, so a new relocating step inherits the upkeep. P2: per-path regression tests (AM-migration-delete-drops-comments). P3: add id-keyed side stores to the raw-store exception obligations in CLAUDE.md, next to version capture.'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

## Problem

Data-migration steps delete on the raw store, below the entitymanager:

- The face move deletes the source face with `DeleteEntityState`
(`internal/datamigration/steps.go`, around line 586).
- The delete step calls `DeleteEntity` (around line 1092).

Neither path fires `AliasRewriter`, so comment threads keyed on the deleted face
or entity survive. A face or entity recreated under the same address inherits
the old thread. This is the delete half of BUG-R1PQY9 on a second write path.

A face move could move the thread to the destination face rather than drop it;
that is the decision to make here.

Found in code review of BUG-R1PQY9 (RR-EST0W6).

## Analysis (current develop)

The paths that remove or relocate a row below the entitymanager:

| Path | Entry points | Store call | Comment effect |
|---|---|---|---|
| `applyFaceMove` (via `applyMoves`) | `rename_face`, `migrate_face`, `rela migrate adopt-face` | `DeleteFace` on the source face | Source thread stranded |
| `dropEntitiesStep.Run` | `drop_entities`, GC sweep (`entity_type` ledger entries, server and `rela migrate gc`) | `DeleteFamily` | Every face's thread stranded |

`rename_entity_type` and the property/relation steps do not change a row's
address, so they do not affect comment keys.

## Decision: a face move MOVES the thread

A face move relocates the same content to a new coordinate; it is not a
deletion. `migrate_face` and `adopt-face` take rows from the zero coordinate,
where every comment written before the type declared faces sits, onto the face
that content now lives at. Dropping the thread would erase commentary because of
a schema change. On a collision (a comment id already at the destination) the
destination copy wins, matching `MergeThreads`.

`drop_entities` deletes the content, so it drops every face's thread, as
`Service.EntityDeleted` does for an entity delete.

## Fix plan

1. `comments.Service.FaceMoved(ctx, type, id, from, to)`: copy each comment
not already at the destination (`Get` miss, then `Add`, which keeps every
field), then `DeleteTarget` the source. Idempotent, so a re-run converges.
2. `datamigration` declares a consumer-side `CommentThreads` interface
(`FaceMoved`, `EntityDeleted`). Optional on `Deps`, `GCDeps` and `AdoptDeps`;
nil means the project has commenting disabled.
3. `applyMoves` moves each batch's threads BEFORE the batch's row
transaction; `dropEntitiesStep` drops the threads BEFORE `DeleteFamily`. Doing
it after would strand the thread on a crash between the two, because a re-run no
longer lists a row that is gone. This is the same ordering the version capture
already uses.
4. Wire `svc.Comments()` into the CLI (`migrate data`, `migrate gc`,
`migrate adopt-face`) and the server GC sweep, converting a nil
`*comments.Service` to a nil interface.

## Regression tests

- `comments`: `FaceMoved` moves a thread with fields intact, merges into an
occupied destination, is idempotent, and leaves other faces alone.
- `datamigration`: `rename_face`, `migrate_face`, `adopt-face` and
`drop_entities` (runner and GC) each leave the thread at the right address.
