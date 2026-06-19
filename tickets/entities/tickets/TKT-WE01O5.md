---
id: TKT-WE01O5
type: ticket
title: 'Two-way fsstore↔pgstore sync: hash-based push/pull with manual conflict resolution'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Goal

Implement two-way sync between a local fsstore repo and a remote pgstore
rela-server, per FEAT-NJ9FEN. Push is the primary path; pull is in scope.
Conflicts are **manual-only** in this ticket — no auto-merge.

## In scope

1. **Client sync index** (`.rela/sync-state.json`): per-record content hash +
opaque server cursor. Local dirty-detection by recomputing the hash.
2. **Push**: `PUT /sync/<kind>/<id>` with `If-Match: <hash>`, applied via
`entitymanager`. Distinct `200` / `412` (conflict) / `422` (invalid) handling.
3. **Pull**: `GET /sync/manifest?cursor=<blob>` → `{changes:{id→hash|null},
cursor}`. Tombstones (`null`) mirror deletes. Diff against index; over-delivery
is a no-op via hash equality.
4. **pgstore manifest support**: the server must produce the manifest cheaply.
**Add a content-hash column to pgstore (and/or reuse `seq`) so the manifest is a
column read, not a full read-and-rehash per pull.** Decide hash vs seq vs both
during planning. Whatever the cursor is built on must be updated by **every**
write path (all six op types, incl. relation delete and rename) and must emit
**tombstones** for deletes — otherwise the delta silently skips changes. (See
FEAT-NJ9FEN load-bearing assumption #3 and the existing `seq > watermark`
overlap-window pattern in the pg watcher.)
5. **Canonical serialization check**: confirm fsstore and pgstore serialize a
record byte-identically before hashing, or normalize so they do. If they
diverge, every push 412s and every pull shows phantom diffs (assumption #1).
6. **Manual conflict resolution via CLI** (keep dumb):
   - `rela sync push` / `rela sync pull` — normal sync; halt + report on conflict.
   - `rela sync push --force <id>` — local wins; overwrite remote, re-baseline.
   - `rela sync pull --force <id>` — remote wins; overwrite local, re-baseline.
Force bypasses the `If-Match` / both-dirty guard.

## Explicitly OUT of scope (deferred)

- Three-way / per-property auto-merge (no base-content storage). Manual force
flags are the only resolution here.
- CRDTs (rejected — bypass validation).
- A client-visible `seq` cursor (kept opaque; server-internal).
- fs↔fs sync (no pg server). The opaque-cursor design keeps the door open but
it's not built here.
- Full-fidelity export/import as a separate deliverable — only ensure the
per-record content transferred by sync is lossless (entity Content + relation
properties/content included).

## Acceptance

- A laptop fsstore repo can push local creates/updates/deletes to a pgstore
server and pull server-side creates/updates/deletes back, converging both ends.
- Concurrent edits to the same record are detected and **halt with a clear
report**; `--force` resolves them deterministically and re-baselines.
- The pgstore manifest is produced from a stored hash/seq column, not a full
rescan, and reflects all write paths including deletes (tombstones).
- Pushed writes go through `entitymanager` (ACL/validation/automation/audit
observed); none bypass it.

## Notes

Design rationale and prior-art survey (CouchDB `_changes`/`_revs_diff`,
PowerSync upload-through-backend, Dolt cell-level merge, Replicache rebase,
rejected CRDTs) live in FEAT-NJ9FEN and the brainstorm. Start hash-only +
full-manifest; the cursor lets the server add an O(changes) delta later without
touching the client.
