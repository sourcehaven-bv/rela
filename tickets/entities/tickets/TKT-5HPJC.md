---
id: TKT-5HPJC
type: ticket
title: Whole-file seal/unseal at fsstore I/O boundary
kind: enhancement
priority: high
effort: m
status: backlog
---

Wire the ported `internal/encryption` primitives into `internal/store/fsstore` so that every entity and relation markdown file is sealed as a whole blob. Depends on TKT-7XEFM landing first.

## Wire format (proposed)

Sealed file layout (bytes):

```
magic (4B, "RLAE")          literal
version (1B, 0x01)          format version
envelope_len (4B, big-endian)
envelope (YAML bytes, the _encryption config reference for this repo)
sealed_payload               AES-256-GCM ciphertext of the full cleartext markdown
```

The `.rela/encryption.yaml` envelope holds the wrapped data keys and the recipient-set fingerprint. Each sealed file MAY inline its own envelope snapshot (for rotation detection), or reference the repo-level envelope by fingerprint — pick one in planning.

## Integration boundary

- **Read**: `fsstore.readEntityFile(path)` peeks first 4 bytes. If `RLAE`, delegate to unseal; otherwise parse as cleartext (backward compat).
- **Write**: after marshalling entity/relation to markdown bytes, if `Crypto != nil`, seal before writing.
- **No changes** to marshalling, YAML parsing, or entity/relation types.

## Keyring divergence check

On every write, compare the recipient-set fingerprint in the repo envelope against a fresh fingerprint computed from the loaded keyring. Mismatch → refuse write with `ErrKeyringDiverged` pointing at the rotation CLI (follow-up ticket).

## Acceptance criteria

1. Entity/relation read paths transparently unseal when `RLAE` magic is present.
2. Entity/relation write paths seal when `Crypto != nil`; otherwise bytes go to disk unchanged.
3. `Crypto == nil` path is byte-for-byte identical to develop (backward compat test).
4. A cleartext repo can be migrated to encrypted by running a one-shot "seal everything" operation (could be a CLI ticket, or just a helper).
5. Reading an encrypted repo without a matching private key fails with a clear `ErrNoMatchingKey`-derived error at load time (not silently empty).
6. Tampered sealed file surfaces `ErrDecrypt` (not silently empty or "unknown format").
7. Metamodel.yaml is NOT sealed.

## Out of scope

- Key rotation / recipient add/remove CLI (separate ticket).
- Cross-repo migration tooling.
- Selective / per-file cleartext exceptions.
