---
id: repo-encryption
type: concept
title: Repository-Level At-Rest Encryption
summary: 'Whole-file sealing of entity and relation markdown using age (filippo.io/age) as the envelope. One data key per repo, wrapped once per recipient. The metamodel, templates, and groups.yaml stay cleartext. Every file under entities/, relations/, attachments/ is sealed as an atomic age blob. Partial encryption is forbidden: fsstore refuses to open a repo where an encryption config exists but any data file is cleartext, or vice versa. v1 uses age''s built-in X25519 recipients; post-quantum hybrid is a follow-up via an age recipient plugin.'
description: |-
    ## Design goals

    - **Coverage**: 90%+ of use cases where the project wants its contents private; no mixed-sensitivity data-leak edge cases.
    - **Simplicity**: delegate envelope format, wrap/unwrap, AEAD, and versioning to age. Our code concerns itself with *where* sealing happens (fsstore I/O boundary) and *when* (based on repo configuration), never with cryptographic primitives.
    - **Boundary**: sealing happens at the fsstore I/O boundary — read a file, try-parse as age, fall back to cleartext only in an unencrypted repo; write flow marshals first, then seals. No frontmatter-vs-body split, no property-level syntax.
    - **Metamodel stays cleartext**: schemas and groups.yaml are bootstrap data; tooling needs them readable.
    - **Recovery escape hatch**: users can run `age -d -i ~/.config/rela/key <file>` to inspect a sealed file without rela.

    ## In scope (sealed)

    - `entities/**/*.md`
    - `relations/**/*.md`
    - `attachments/**/*` (treated as opaque binary through the same age envelope)
    - `.rela/cache.json` and `.rela/fsstore-index.json` (derived cleartext on disk would defeat the threat model)

    ## Out of scope (cleartext)

    - `metamodel.yaml`, `groups.yaml`, `schedules.yaml`, `data-entry.yaml`
    - `templates/`
    - `.rela/encryption.yaml` (the age recipient list — this is public routing info)
    - `<repo>/keys/*.pub` (age public keys — by definition public)

    ## Threat model

    Protects against:
    - Casual disk read
    - Accidental git push to a public remote
    - Stolen laptop without a loaded private key
    - Read-only disk access to a CI runner cache

    Does NOT protect against:
    - Recipients who decrypted past files keeping copies (cryptography cannot enforce forgetting).
    - Size correlation: file sizes are visible and roughly correlate with plaintext length (age overhead is fixed). Documented-and-accepted, not mitigated with padding.
    - Subprocess inheritance of `$RELA_KEY_FILE`: prefer `.rela/key` or `~/.config/rela/key` for locked-down setups where subprocesses should not see the private key path.
    - Modification time correlation: an adversary can see when a file changed.
    - Active attackers with write access to `<repo>/keys/`: they can substitute pubkeys and the tool cannot detect it without signing (out of scope for v1).
    - Leaks through the MCP server or Lua scripts: consumers of the unsealed data stream see cleartext as always; encryption is at-rest only.

    ## Key rotation semantics

    - **Add member**: add their pubkey to `<repo>/keys/`. The rotation command rewraps the existing data key for the new recipient. File bodies are NOT rewritten. New recipient gains read access to history.
    - **Remove member**: the rotation command generates a fresh data key, re-encrypts every sealed file, drops the old recipient. Removed member retains access to any file they already read and copied locally (cryptography cannot prevent this).
    - **Partial encryption forbidden**: on `fsstore.New`, if `.rela/encryption.yaml` exists, every file under entities/, relations/, attachments/ must be sealed. If the marker file is missing but sealed files are present (or vice versa), fsstore refuses to open with a pointer at the rotation CLI. The one-shot seal/unseal migration is a first-class CLI command with resumable state and an `.rela/migration.inprogress` marker.

    ## What this replaces

    A prior per-group/per-property design (metamodel `encrypted:` declarations, groups.yaml, _enc_v1_ property prefix, _encryption YAML block) plus a subsequent custom-envelope per-repo design. Both superseded: the group design leaked across properties in the same file, and the custom envelope duplicated work age has already done.

    See DEC-D5P4X for the decision to use age.
package: internal/encryption
layer: infra
status: draft
---
