---
id: TKT-GFJJ3S
type: ticket
title: 'Sync 2/5: pgstore deletion tombstones + seq indexes + manifest query'
kind: enhancement
priority: medium
effort: m
status: ready
---

Sub-ticket of TKT-WE01O5 / FEAT-NJ9FEN.

## Problem

pgstore deletes are HARD deletes (`entity.go:324`, `relation.go:217`); the `seq
> X` catch-up reads live rows only (`listener.go:232-239`) and `catchUpEvent`
only emits Updated, never Deleted (`listener.go:276-281`). A seq-based manifest
therefore CANNOT discover deletions. Also `seq` is UNINDEXED — `WHERE seq > X`
is a seqscan+sort today.

## Scope

- Migration `000X_sync.sql`: NEW `deletions` tombstone table
(`kind, id_a, id_b, id_c, seq BIGINT DEFAULT nextval('rela_seq'), deleted_at`).
- Insert a tombstone row in the SAME tx as each DELETE
(`entity.go` entity + cascade-relation delete, `relation.go` relation delete).
- B-tree indexes on `entities(seq)`, `relations(seq)`, `deletions(seq)`.
- Manifest query helper: live `(id, seq>X)` UNION tombstones `(seq>X)`.
- Extend `listener.go` catch-up to scan tombstones and emit Deleted events
(so the durable path is no longer delete-blind).

## Acceptance

- Delete an entity/relation → a tombstone row exists with a fresh seq.
- Manifest `seq > X` returns changed live rows AND tombstones; EXPLAIN shows an
index scan on `seq`, not a seqscan.
- Missed-NOTIFY recovery: a delete that misses the live NOTIFY is still recovered
by the seq catch-up (the bug this fixes). Test explicitly.
- Delete-then-recreate same id: both tombstone and live row appear past the cursor.
- pgstore storetest conformance still passes; postgres build doesn't link bleve.

## Notes

Soft-delete (deleted_at flag on the live row) was REJECTED in planning — it
forces `deleted_at IS NULL` filters across every read/list/search/cascade path.
Tombstone table keeps existing reads untouched.
