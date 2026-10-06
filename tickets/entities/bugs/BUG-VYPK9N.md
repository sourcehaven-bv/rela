---
id: BUG-VYPK9N
type: bug
title: Text search misses faced entities after a search index rebuild
description: 'On the file and sqlite backends, a text search does not find an entity of a faced type until that entity is written again. In the markdown editor, a new task body with `@proc` offers the `procedure` type, but a search under it (`@procedure:Offb`) says No matches, although the procedure Offboarding exists in the world the page reads. The command palette, the entity picker, list search and Lua `rela.search` read the same index and have the same gap. Expected: a text search finds the face the world serves, also right after startup.'
priority: high
effort: s
why1: The bleve backend drops the hit. Its document key is the bare id, so resolveHits reads it as the default face, and the world serves a named face instead.
why2: IndexBatch, which the startup backfill uses, keys each document by e.ID. EntityPut, the per-write path, keys by docKey(e.ID, e.Face). Only a later write fixes the key.
why3: Per-face document keys were added to EntityPut, EntityRenamed and the delete paths, but IndexBatch builds its key on its own and was missed.
why4: Tests of world-scoped search seed the index through EntityPut or use the linear backend. No test searches a faced entity after a backfill.
why5: The index has no format version. A change to how documents are keyed cannot invalidate an index written the old way, and a persisted index that looks current is never rebuilt.
prevention: Every write path builds the key with docKey. A test pins that a backfilled faced entity is found in its world. The index stores a format version, and opening an index with an older version rebuilds it.
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

## Reproduction

Environment: `rela-server` on a file project (bleve search index).

```yaml
entities:
  task:
    properties: { title: { type: string } }
  procedure:
    faces:
      concept: { label: Concept }
      adopted: { label: Adopted }
    properties: { title: { type: string } }
worlds:
  current:
    select: [adopted, concept]
    otherwise: default
```

Seed `entities/procedures/PROCEDURE-1@adopted.md` with title `Offboarding`,
start the server, then:

1. `GET /api/v1/_search?q=type:procedure` lists PROCEDURE-1.
2. `GET /api/v1/_search?q=Offb` returns nothing.
3. Save PROCEDURE-1@adopted once through the API. Now step 2 finds it.

In the UI: on a new task, type `@proc` in the body, choose `procedure`, type
`Offb`. The menu says No matches.

The postgres backend searches in SQL and is not affected.

## Root cause

`bleveindex.Index.IndexBatch` writes `batch.Index(e.ID, doc)`. Every other write
path keys a document by `docKey(e.ID, e.Face)`, the state ref
(`PROCEDURE-1@adopted`). After a backfill, a faced entity's document has the
bare key `PROCEDURE-1`, and two faces of one entity overwrite each other.
`resolveHits` parses the bare key as the default face. The world resolves the
entity to `adopted`, the faces differ, and the hit is dropped as non-prime.

The `type:` listing reads the store, not the index, so the scoped starting list
still shows the entity. Only text search misses it.

## Fix plan

1. `IndexBatch` keys with `docKey(e.ID, e.Face)`.
2. The bleve index stores a format version. `New` discards an index with no
or an older version and creates a fresh one, so the startup backfill rewrites
it. Without this, an index the old code wrote stays "current" and keeps the bare
keys.
3. Tests: a bleveindex test that a batch-indexed faced entity is found in a
world; a test that an index without the current format version is rebuilt on
open; the e2e spec `faces-mention.spec.ts` for the `@` menu.

Related areas checked: `EntityRenamed` gets one call per face from every store
(storetest observer contract). The postgres backend searches in SQL and is not
affected. The linear backend is in-memory and rebuilt on start through
`EntityPut`.
