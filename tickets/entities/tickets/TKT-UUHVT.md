---
id: TKT-UUHVT
type: ticket
title: Remove attachment CAS/dedup; add streaming I/O
kind: refactor
priority: high
effort: m
status: ready
---

## Problem

Today `internal/attachment/Store` writes uploaded files to
`attachments/<hash-prefix>/<hash>.<ext>` — filename is the SHA-256 of the
plaintext, two-char prefix directory for fanout. A per-file YAML sidecar records
the original filename, content type, size, uploader.

The encryption PR (#464) routes these writes through cryptofs so the bytes on
disk are sealed, but two residuals remain:

1. **Plaintext-hash filename is a guessing oracle.** Anyone with
disk read access can hash a candidate file and confirm its presence in the repo,
even though the contents are sealed.
2. **Dedup precludes streaming.** The hash isn't known until every
byte is in memory, so `AttachFile` does `io.ReadAll` → hash →
`WriteFile(derived-path, bytes, ...)`. A 500 MB PDF uses 500 MB of plaintext
heap (+ another 500 MB of ciphertext during seal). Streaming through age — which
is natively stream-shaped — would cap peak memory at one 64 KiB chunk.

Secondary motivations:

- Dedup is low-value for rela's workload (design docs, screenshots,
spec PDFs — real-world duplication within one repo is rare).
- Dedup complicates GC (walk CAS directory, cross-reference every
entity property). Without dedup, attachments are 1:1 with an entity property;
entity delete → directory delete.
- Dedup complicates concurrent writes (two parallel uploads racing
on the same hash path).

## Proposed design

Replace the CAS with a per-entity/per-property layout:

```
attachments/<entity-id>/<property>/<original-filename>
```

- No content-visible filename. Leaks entity ID + property name
(already leaked elsewhere) but not content.
- Streaming natural: `OpenWrite(path)` returns an `io.WriteCloser`
that pipes through `age.Encrypt` directly. Fixed 64 KiB peak memory.
- 1:1 ownership — delete the entity, delete the directory. No GC
pass needed.
- Original filename IS the filename on disk. No sidecar YAML in
the basic case.

## Content type

The sidecar currently records MIME type for `Content-Type` on downloads. Replace
with `mime.TypeByExtension` on the fly (`application/octet-stream` fallback).
Re-surface content type as an entity property only if we find a case where
extension-based inference is wrong in user-visible ways.

## Out of scope

- Backend-layout refactor (`internal/backend/{fs,mem}/...`). This
ticket stays in the current layout; the dedup removal stands alone. See
`.ignored/backend-layout-refactor.md` for that work.
- Changing the file-type property schema (still a bare string
holding the path).

## Migration

None needed — the encryption feature is unreleased, no on-disk data to migrate.

## Related

- `encryption-security-review.md` C1: attachments sealed via
cryptofs (shipped in #464, imperfect — this ticket is the proper fix).
- `encryption-security-review.md` S4: sidecar YAML plaintext
(resolved in #464; disappears entirely when CAS is removed).
- `.ignored/attachment-dedup-removal.md`: full design notes and
rationale.
