---
id: FEAT-SQLBK1
type: feature
title: SQLite storage backend for single-process deployments
summary: The embedded single-file backend decided by DEC-LFSYNY — pgstore's transaction tier at fsstore's operational cost, for desktop and single-server deployments.
description: 'A store.Store on modernc.org/sqlite (pure Go, no cgo) sitting between fsstore and pgstore: indexed queries and real transaction rollback without a database server, giving up git-diffable markdown and cross-process serialization. Single-process is enforced by an exclusive sidecar lock rather than documented and hoped for, because rela enforces `unique:` with an untransacted scan and two writers would have no backstop. Built by DEC-LFSYNY; this entity exists so work on the backend has something to hang off.'
priority: medium
status: implemented
---

## Why this entity exists

DEC-LFSYNY decided the backend and `internal/store/sqlitestore` implements it,
but no `feature` was ever created — so tickets and bugs touching the backend
had nothing to link to, and BUG-LL3C07 could not satisfy the `fixes`
cardinality rule without pointing somewhere unrelated.

## Scope

What DEC-LFSYNY licensed and this covers:

- `store.Store` on `modernc.org/sqlite`, at pgstore's transaction tier — real
  rollback, events withheld until commit.
- Single-process enforcement via an exclusive sidecar lock, refusing the second
  process rather than admitting it.
- Refusal to open where WAL cannot be enabled (iCloud, Dropbox, SMB), because
  SQLite is unsafe there and the sidecar lock is unreliable too.
- Attachments as BLOBs in the same database.
- Pairing with bleve rather than FTS5: `search.Visible` wraps any `Searcher`,
  so a native FTS5 backend is an optimization, not a prerequisite.

Explicitly **not** licensed by that decision, and so not part of this feature:
multi-process SQLite, replacing pgstore for shared servers, FTS5 search
integration, SQL pushdown, `ManifestSince`. Each needs its own decision.

## Known gaps

- No content versioning yet, so this backend currently has neither git history
  nor version history — the one place it is behind both of its neighbours.
- `synchronous=NORMAL` means a crash can lose recently committed transactions,
  a deliberate durability trade to revisit if it proves wrong.
