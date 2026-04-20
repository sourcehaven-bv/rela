---
id: TKT-UUHVT
type: ticket
title: Remove attachment CAS/dedup; add streaming I/O
kind: refactor
priority: high
effort: m
status: ready
---

## Status: READY (2026-04-20, unblocked by encryption rollback)

The encryption rollback (PR #508) has merged. The blocker on this ticket is
cleared: cryptofs is gone, so the attachment refactor is now the simple
~300-line change originally envisioned, not the 50-file refactor it would have
been alongside cryptofs.

The previous in-progress implementation was dropped — the git stash that held
the ~90%-complete cryptofs-coexisting version has been discarded. Next pickup
starts fresh against today's tree.

## Problem

Today `internal/attachment/Store` writes uploaded files to
`attachments/<hash-prefix>/<hash>.<ext>` — filename is the SHA-256 of the
plaintext, two-char prefix directory for fanout. A per-file YAML sidecar records
the original filename, content type, size, uploader.

Two residuals from the legacy design:

1. **Plaintext-hash filename is a guessing oracle** (historical — mattered
for the encryption threat model; less relevant post-rollback but still a weak
signal to any filesystem observer).
2. **Dedup precludes streaming.** `AttachFile` does
`io.ReadAll → hash → WriteFile`. A 500 MB PDF uses 500 MB of plaintext heap
before the first byte hits disk.

Secondary motivations:
- Dedup is low-value for rela's workload (design docs, screenshots,
spec PDFs — real-world duplication within one repo is rare).
- Dedup complicates GC (walk CAS directory, cross-reference every
entity property). Without dedup, attachments are 1:1 with an entity property;
entity delete → directory delete.
- Dedup complicates concurrent writes.

## Proposed design

Replace the CAS with a per-entity/per-property layout:

```
attachments/<entity-id>/<property>/<original-filename>
```

- No content-visible filename. Leaks entity ID + property name
(already visible elsewhere) but not content.
- Streaming I/O natural: `os.OpenFile` + `io.Copy`. Fixed-memory.
- 1:1 ownership — delete the entity, delete the directory. No GC
pass needed.
- Original filename IS the filename on disk. No sidecar YAML.

## Scope (simplified post-rollback)

In scope:
- Replace `internal/attachment/Store` with `store.AttachmentManager`
(per-entity layout, already implemented in fsstore + memstore as dead code).
- Rewrite `fsstore.AttachFile` / `ReadAttachment` to stream. Add
`OpenWriteStream` / `OpenReadStream` to `storage.FS` — or just use
`os.OpenFile`/`os.Open` directly from `fsstore` since there is no transform
layer to route through.
- Delete `internal/attachment/` package.
- Update workspace: `AttachFile`, `ListAttachments`; drop
`GCAttachments`.
- Update CLI: `rela attach` help, `rela detach` drops hash-prefix
arg, `rela attachments` drops `OriginalName` column, `rela gc` drops
`--attachments` flag (keeps `--temp-files`).
- `fsstore.DeleteEntity` cascades `attachments/<entityID>/`.
- `fsstore.RenameEntity` moves `attachments/<oldID>/` → `<newID>/`.
- Regenerate attachment-path fixtures in tickets and docs.

Out of scope:
- Backend-layout refactor (`internal/backend/{fs,mem}/`) — separate
ticket.
- Changing the file-type property schema (still a bare path string).
- Multiple files per property (contract is one file per
`(entityID, property)`).

## Migration

None — attachment feature is unreleased, no on-disk data to migrate.

## What changed vs the original (pre-pause) plan

Reasons this is smaller than what was in flight before:
- No cryptofs streaming writer needed. `fsstore.AttachFile` writes
through the single FS handle.
- No header-stamping wrapper. No sealed-blob format to coexist with.
- No `ErrRenameEntityNotSupportedOnEncryptedRepo` guard. Rename
works on any repo.
- No `bytesFS` / `BytesFS` / `bytesOpener` wiring. Single `storage.FS`
everywhere.
- No two-handle fsstore split to thread attachments through.

## Related

- `.ignored/attachment-dedup-removal.md` — full design notes and
rationale (still accurate).
- PR #508 (MERGED) — encryption rollback that unblocked this ticket.
