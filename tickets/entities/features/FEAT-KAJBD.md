---
id: FEAT-KAJBD
type: feature
title: At-rest encryption for entity and relation files (whole-repo)
summary: Seal every entity, relation, attachment, and derived-cache file transparently at the fsstore I/O boundary using age (filippo.io/age). One age recipient list per repo. metamodel.yaml and templates stay cleartext. Public keys live in <repo>/keys/<name>.pub as age public keys. Local private key resolved via $RELA_KEY_FILE → .rela/key → ~/.config/rela/key. Partial encryption is architecturally forbidden.
description: |-
    ## User-facing behavior

    When the repo is encryption-enabled (`.rela/encryption.yaml` exists):
    - Entity/relation/attachment files on disk are age blobs (`age-encryption.org/v1` header). They are not human-readable.
    - `rela <any command>` reads transparently for any recipient with a usable local age identity.
    - Writes always seal; there is no cleartext escape hatch.
    - `metamodel.yaml`, `groups.yaml`, `schedules.yaml`, `data-entry.yaml`, and `templates/` stay cleartext (bootstrap config).
    - `age -d -i ~/.config/rela/key <file>` works as a diagnostic escape hatch without rela.

    When the repo is not encryption-enabled:
    - Filesystem layout and file bytes are byte-for-byte identical to pre-feature behavior.
    - fsstore installs an `identityCrypto` that no-ops seal/unseal; there is no `if crypto != nil` branch.

    ## Key management

    - `<repo>/keys/<name>.pub` — recipient age public keys (committed)
    - `.rela/encryption.yaml` — the recipient list (committed; public routing info, no secrets)
    - `.rela/key` or `~/.config/rela/key` or `$RELA_KEY_FILE` — local age identity (NOT committed)

    ## Recipient management (CLI, follow-up ticket)

    - `rela keys generate [name]` — write age pub/priv pair
    - `rela keys add <name>` — rewrap-by-rewriting: re-encrypt every file so the new recipient can read
    - `rela keys remove <name>` — re-encrypt every file under the new recipient list (removed member retains access to any files they already read locally; cryptography cannot forget)
    - `rela keys rotate` — same as remove-then-add for key-compromise scenarios
    - `rela keys sync` — detect drift between committed recipients and actual seal-target of files; offer to fix

    ## Partial-encryption invariant

    On `fsstore.New`:
    - If `.rela/encryption.yaml` exists: every file under entities/, relations/, attachments/, and every derived cache file (`.rela/cache.json`, `.rela/fsstore-index.json`) MUST be an age blob. Any cleartext file causes `fsstore.New` to fail with a clear error pointing at `rela keys migrate`.
    - If `.rela/encryption.yaml` does not exist: every data file MUST be cleartext. Any age blob causes `fsstore.New` to fail.
    - This invariant forbids ambiguous half-migrated states that could silently commit secrets.

    ## Recipient drift warning (not an invariant)

    On fsstore open, compare the committed recipient list against the loaded keyring. If they diverge (added pubkey not yet wrapped into existing files, or removed pubkey still in wraps), emit a startup warning pointing at `rela keys sync`. This is NOT enforced at write time and is NOT a cryptographic protection; it is a UX affordance for forgotten rotations.

    ## What this does NOT do

    - Does not cryptographically enforce that a removed recipient cannot read files they already copied.
    - Does not hide file names, sizes, or modification times.
    - Does not prevent MCP / Lua consumers from seeing cleartext (encryption is at rest only).
    - Does not sign or authenticate the recipient list against tampering (future: signed envelopes).
priority: high
status: proposed
---
