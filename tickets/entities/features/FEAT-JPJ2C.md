---
id: FEAT-JPJ2C
type: feature
title: At-rest encryption for entity properties and bodies
summary: Hybrid post-quantum encryption for selected entity/relation fields and bodies, configured in the metamodel with group-based access control.
description: 'Lets rela projects protect sensitive content in shared git repositories. Properties and bodies marked encrypted in metamodel.yaml are encrypted with AES-256-GCM using a per-file data key. The data key is wrapped per recipient with hybrid X25519 + ML-KEM-768 (quantum-resistant). Recipients are organised into groups so different fields can be readable by different team members. Private keys live outside the repo; public keys are committed to keys/. The work is built up in slices: crypto primitives first, then metamodel integration, store integration, key management commands, and finally wiring into MCP / data-entry / desktop.'
priority: medium
status: proposed
---

## Motivation

rela projects live in git repositories. When the repository is shared (public,
cloud-hosted, accessible to external contractors, or replicated across
environments), any sensitive content in entities or their bodies is exposed.
Today the only option is to not track sensitive artefacts in rela at all — which
defeats the traceability goal.

## Strategy

**Metamodel-driven, transparent at runtime.** Users declare which properties and
bodies are encrypted in `metamodel.yaml`. The store layer encrypts on write and
decrypts on read; MCP / data-entry / desktop see cleartext when a private key is
available. No encryption changes to existing entity operations from the user's
perspective.

**Hybrid post-quantum from day one.** Each file's AES-256-GCM data key is
wrapped per recipient using X25519 + ML-KEM-768. Both must be broken to
compromise the key. This hedges against both classical and quantum adversaries
and follows the pattern adopted by Signal, Chrome, and Apple iMessage.

**Groups, not per-person.** Recipients live in `keys/*.pub`; groups map group
names to lists of recipient filenames. Metamodel references group names. Adding
a person to a group gives them access to new encrypted files automatically; lazy
re-encryption brings existing files up-to-date on the next write.

## Slices

1. **Crypto primitives** (`internal/encryption/`) — keygen, hybrid wrap/unwrap, AES-256-GCM encrypt/decrypt, PEM format, keyring that loads recipients from `keys/` and the private key from the standard locations. Pure library, no rela coupling.
2. **Metamodel integration** — parse `encrypted: <group>` on properties/bodies; parse groups config; resolve which fields of which entity types need encryption.
3. **Store integration** (`internal/store/fsstore/`) — on write, encrypt marked fields before serialisation; on read, decrypt when key is available, leave ciphertext blobs present otherwise. `!enc` YAML tag serialisation.
4. **CLI: `rela keys generate`** — generate a hybrid keypair, write PEM private key to default location, print public key for the user to commit to `keys/`. Plus a settings panel equivalent in the data entry UI.
5. **Lazy re-encryption + rotation** — per-file key version; `rela keys rotate-group <name>` for explicit full re-encryption; automatic re-encrypt on write if key version stale.
6. **Docs, migration, onboarding** — guide for setting up encryption in an existing project, documentation on threat model and limitations.

MCP, data-entry, and desktop automatically pick up encrypted data through the
store layer — they don't need their own encryption logic.

## Design Principles

- **One wire format**: AES-256-GCM for data, hybrid X25519 + ML-KEM-768 for key wrapping. No per-provider pluggability.
- **No new file layout**: `!enc` tag inside existing YAML frontmatter, optional `_encryption` block for wrapped data keys.
- **Cleartext graph**: entity structure, relations, and non-encrypted properties always visible.
- **Transparent at runtime**: no special commands for reading encrypted data; the store handles it.
- **Opt-in**: projects without encryption configured are completely unaffected.

## Explicit Non-Goals

- Passphrase-protected private keys (first slice — threat model is remote storage, not local filesystem)
- Encrypted cache / graph (handled after the in-progress store refactor lands)
- Search over encrypted content
- Hiding entity existence, relation topology, or non-encrypted properties
- Scrubbing git history on key rotation
- MAC over cleartext fields (deferred — per-value GCM tags provide integrity for encrypted data)

## Open Questions

- Exact serialisation of wrapped data keys (raw PEM block? base64? length-prefixed?)
- Behaviour when writing without access to the encryption group (error vs fallback)
- Should `list` / `trace` surface encrypted fields as `<encrypted>` placeholder or omit them entirely?
- Handling of entities moved between encryption groups over time
