---
id: FEAT-NJ9FEN
type: feature
title: Two-way sync between local fsstore and remote pgstore repos
summary: Hash-based two-way fsstore↔pgstore sync with opaque pull cursor and manual conflict resolution.
description: 'Two-way sync between a local fsstore rela repo and a remote pgstore rela-server. A per-record content hash is the single token: it detects local changes and serves as the If-Match precondition for conditional push through entitymanager. Pull uses a manifest of {id -> hash | tombstone} keyed by an opaque server-minted cursor (server-internal; may be backed by a hash/seq column for O(changes) deltas later). Conflicts are detected (412 on push, both-dirty on pull) but resolved manually via rela CLI force flags. CRDTs and three-way auto-merge are rejected/deferred.'
status: proposed
---

## Summary

Sync a local fsstore rela repo (markdown on a laptop) with a remote pgstore rela
server: push local changes up and pull remote changes down. Crosses the fs↔pg
boundary, so the protocol speaks rela's logical entity/relation model, not files
(git-as-transport is out — one end is Postgres rows, not a checkout).

## Design (agreed in brainstorm)

**Tokens & index.** The client keeps a sync index (`.rela/sync-state.json`): `{
records: { id → content-hash }, cursor: <opaque server blob> }`.

- **Content hash** is the single per-record token. It does double duty:
  - **Local dirty detection** — recompute hash of the working file; differs from
the index → locally changed → needs push. (fsstore has no logical clock, so the
hash is the only way the laptop knows its own files moved.)
  - **Conditional-write precondition** — push sends `If-Match: <index-hash>`; the
server applies only if its current content still hashes to that value.
- **Opaque cursor** — the server mints a cursor and the client stores/echoes it
verbatim, never parses it. The server decides what's inside (full-scope marker,
`seq`, timestamp+overlap, LSN, …) and can change that unilaterally, even per
backend. Day one the server may ignore it and return the full manifest; later it
can back it with a `seq`/hash column for an O(changes) delta — with **no change
to the client protocol or index format.**

**Push.** Per locally-dirty record: `PUT /sync/<kind>/<id>` with `If-Match:
<index-hash>`, full content body. Applied **through `entitymanager`** (gets ACL,
validation, automations, audit — matches PowerSync's "upload through the backend
write API" model; never raw rows). Responses:
- `200` + new hash → applied; client updates index hash.
- `412 Precondition Failed` → remote moved since base → **conflict**.
- `422` → entitymanager rejected the content (validation/ACL) → **not** a
conflict; fix the data. Keeping 412 (stale base) and 422 (invalid content)
distinct is a hard requirement.

**Pull.** `GET /sync/manifest?cursor=<stored>` → `{ changes: { id → hash | null
}, cursor: <new> }` where `null` is a tombstone (delete). Client diffs each
entry against the index:
- absent-locally / hash differs, local clean → fetch content, apply via local
entitymanager, update index.
- `null` → mirror the delete.
- hash equals index → no-op (re-delivered; harmless — the hash makes an
over-delivering cursor safe).
- hash differs **and** local also dirty → both moved since base → **conflict**.

**Conflict policy (this scope): manual only.** No three-way merge, no
base-content storage, no auto-resolution. On any detected conflict (412 on push,
or both-dirty on pull) the sync **halts for that record and reports it**. The
user resolves by re-running with a force flag on the rela CLI: `rela sync push
--force <id>` (local wins: overwrite remote) or `rela sync pull --force <id>`
(remote wins: overwrite local). Force bypasses the `If-Match` / both-dirty check
and re-baselines the index. Keep this dumb.

## Rejected / deferred

- **CRDTs** — rejected. Bypass validation hooks; even ElectricSQL (CRDT inventors)
abandoned them in their 2024 rebuild.
- **Three-way per-property auto-merge** (Dolt cell-level model) — deferred to a
later feature; only worth it if conflicts prove frequent. The index format is
designed so adding base-content later is additive.
- **`seq` as a client-visible cursor** — deferred; it's a server-internal pull
optimization hidden behind the opaque cursor.

## Load-bearing assumptions (verify during planning)

1. fsstore and pgstore share a **canonical, byte-stable serializer** so a record
hashes identically on both sides — otherwise every push 412s and every pull
shows phantom diffs.
2. pgstore can produce the **manifest cheaply** — ideally a stored content hash
(and/or `seq`) column rather than read-and-rehash every row per pull.
3. The pull feed must include **deletes as tombstones**, and every pgstore write
path (all six op types incl. relation delete/rename) must update whatever the
cursor is built on, or the delta silently skips changes.
