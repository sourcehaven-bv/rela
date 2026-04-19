---
id: FEAT-KAJBD
type: feature
title: At-rest encryption for entity and relation files (whole-repo)
summary: Encrypt every entity and relation markdown file transparently at the fsstore I/O boundary using a single repo-level data key. The metamodel and schema files stay cleartext. Public keys are published in <repo>/keys/ and wraps are stored in .rela/encryption.yaml. Local private key is resolved via the same precedence chain as before ($RELA_KEY_FILE → .rela/key → ~/.config/rela/key). Crypto == nil preserves byte-for-byte cleartext behavior.
description: |-
    ## User-facing behavior

    When the repo is encryption-enabled (`.rela/encryption.yaml` exists):
    - Entity/relation files on disk start with a magic header + envelope, then sealed ciphertext. They are not human-readable.
    - `rela <any command>` reads transparently for any recipient with a usable local private key.
    - Writes always seal — no way to accidentally land cleartext.
    - The metamodel.yaml stays cleartext so `rela` can bootstrap without a key.

    When the repo is not encryption-enabled:
    - No encryption code path runs on reads or writes (byte-for-byte identical to pre-feature behavior).

    ## Key management

    - `<repo>/keys/<name>.pub` — recipient public keys (committed)
    - `.rela/encryption.yaml` — envelope: {magic, version, recipient_fingerprint, data_keys: {name: wrapped_blob}} (committed)
    - Local private key (not committed): RELA_KEY_FILE → .rela/key → ~/.config/rela/key

    ## Recipient management (CLI, follow-up ticket)

    - `rela keys generate <name> [outdir]` — write pub/priv pair
    - `rela keys add <name>` — rewrap data key for new recipient (no file rewrites)
    - `rela keys remove <name>` — generate fresh data key, re-encrypt all entity/relation files, drop old recipient
    - `rela keys rotate` — same as remove-then-add for key-compromise scenarios

    ## Recipient-set fingerprint

    The envelope includes a deterministic fingerprint (e.g., sorted SHA-256 of recipient pubkeys). On every write, fsstore checks that the fingerprint in the current on-disk envelope matches the one it would produce from the live keyring. Mismatch → refuse the write with a clear "keyring diverged; run `rela keys sync`" error. This does NOT make past-read data forgettable — it just makes the forgot-to-rotate footgun loud.
priority: high
status: proposed
---
