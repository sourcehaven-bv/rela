---
id: TKT-RJS3KKJ
type: ticket
title: Erase deleted-entity state from automation job payloads
kind: enhancement
priority: medium
effort: s
started: "2026-11-06"
completed: "2026-11-07"
status: done
description: Seal the on.deleted job payload's last state under a per-job key that is erased on completion and purge; sweep finished postgres job rows.
---

## Description

Follow-up to TKT-Q0AQ840 (PR #2110), from CISO review finding 1 and deferred
review response RR-XA6Y15E. An `on: {deleted: true}` automation job carries the
deleted face's raw last state in its payload, hidden (`visible:`) fields
included. On postgres that payload stays in `neoq_jobs` / `neoq_dead_jobs` with
no retention, `rela history-purge` does not reach it, and a soft-delete purge
does not either. Grounds: PROCEDURE-u020 step 3, POLICY-010 §4, PROCEDURE-rwsq
step 2.

## Approach

- Seal the last state with AES-256-GCM under a fresh per-job key. The payload
holds only a random handle and the ciphertext. The key lives in `state.KV` under
the entity id's hash, next to an index of the entity's handles.
- The job erases its key when it finishes: on success, on a drop, and on its
final attempt. Without the key the ciphertext in the job row is unreadable.
- Purge erases every key for the entity: the soft-delete purge (new
`entitymanager.PurgeNotifier`) and `rela history-purge --commit`.
- The postgres queue deletes finished rows from `neoq_jobs` and rows in
`neoq_dead_jobs` after 24 hours, so the ciphertext itself does not stay. This
also covers jobs the dispatcher drops before the handler runs.

## Acceptance

- Table-driven tests show the sealed state cannot be opened after the job
completes, after a final failure, and after a purge.
- A DB-gated test shows the retention sweep removes finished and dead rows and
keeps pending ones.
- `docs-project/` documents the retention of deleted state.
