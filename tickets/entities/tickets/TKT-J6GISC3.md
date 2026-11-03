---
id: TKT-J6GISC3
type: ticket
title: 'Mentions follow a rename: tombstones on database stores, in-place rewrite on fs'
kind: enhancement
priority: medium
effort: l
started: "2026-11-02"
completed: "2026-11-03"
status: done
---

## Description

A conversion renames the entity family. Neither conversion nor plain rename
updates `` `OLD-ID` `` code-span mentions in other entities' bodies, and mention
resolution does not follow a rename. Mentions break and backlinks (`mentioned-by
`) lose them.

## Approach

Add one guarantee to `store.Store.RenameFamily `: after a rename, a mention of
the old id resolves to the new entity. Rename and convert both get it.

- **postgres and sqlite:** in the same Tx, write a retired-id tombstone: old
id, old type and face, new id, cause (rename or the conversion name). The
tombstone reserves the old id against reuse. Mention resolution and
`mentioned-by ` read tombstones in one batched lookup per page. Writing a
tombstone collapses chains (A→B then B→C leaves A→C). Mention text is not
rewritten.
- **fs and mem:** inside `RenameFamily `, under the write mutex, rewrite the
old id's code spans in other entities' bodies. Find them through the existing
mention index (`store.MentionIndex `) and reuse its parser. Each rewritten
entity emits an update event so search stays current. Like the relation re-key
`RenameFamily ` already does, this runs below the ACL: it replaces an id, it
does not disclose content. Git keeps the history. No id is reserved.
- **ACL:** an old id resolves only when the caller can read the new entity;
otherwise it answers the uniform 404. Tombstones are never listed or served on
their own.
- **Conformance:** `storetest ` asserts that a mention of the old id resolves
after a rename on every backend. Id reservation and text rewriting differ by
backend and are declared as `Capabilities `.
- **Docs:** on fs, a mention added after the rename (merged branch, hand edit)
stays dangling, and a reused id captures it.

## Out of scope

- Reversing a conversion.
- Redirecting old URLs and API GETs by old id. The tombstone lookup makes this
possible; file separately.
