---
id: encryption
type: concept
title: Encryption
summary: At-rest encryption of entity properties and bodies in shared git repositories, using hybrid post-quantum key wrapping and AES-256-GCM.
description: Allows rela projects to protect sensitive entity content when the repository is shared (public git, multi-tenant hosting, etc.). Encryption is configured in metamodel.yaml per property and per body, with group-based access control. Uses AES-256-GCM for data encryption and a hybrid X25519 + ML-KEM-768 scheme for key wrapping — quantum-resistant against both classical and future quantum attacks. Graph structure and cleartext properties remain readable for traceability operations; only opted-in fields are encrypted.
package: internal/encryption
layer: core
status: draft
---

## Purpose

Some rela projects contain sensitive content (compliance analysis, security
decisions, customer data, architectural details) that teams still want to track
using rela's markdown-in-git model. At-rest encryption makes this safe: the
repository can be shared (public git, cloud hosting, external contractors with
scoped access) while designated fields remain opaque to anyone without a
recipient private key.

## Threat Model

**In scope:**

- Remote storage confidentiality (git hosting providers, backups, forks)
- Per-entity-type and per-property access control via groups
- Team key management — adding/removing members without reissuing shared secrets
- Post-quantum resistance (harvest-now-decrypt-later defence)

**Out of scope (explicitly):**

- Local filesystem security — users with filesystem access to the project can still read the private key
- Perfect forward secrecy against compromised private keys
- Hiding graph structure or entity existence (relation files remain cleartext)
- Scrubbing git history after key rotation (history retains previous ciphertexts; removed members keep previous clones)

**Note on passphrase protection:** private keys are intentionally *not*
passphrase-protected in the first slice. The mitigated threat is remote storage,
not local filesystem access.

## Design Pillars

### 1. Metamodel-driven

Encryption is declared in `metamodel.yaml` per entity/relation type. Properties
and bodies carry `encrypted: <group-name>`. A separate groups config maps group
names to recipient filenames in the `keys/` directory.

### 2. Hybrid post-quantum

Each file's AES-256-GCM data key is wrapped for each recipient using a combined
X25519 + ML-KEM-768 envelope. Both must be broken to compromise the wrapped key.
Follows the Signal / Chrome / Apple iMessage pattern.

### 3. Separation of concerns

- `internal/encryption/` — crypto primitives, key generation, wrap/unwrap, encrypt/decrypt, keyring
- `internal/store/fsstore/` — integration point: encryption happens transparently on read/write
- `internal/metamodel/` — parses `encrypted:` declarations and groups config
- `internal/cli/` — `rela keys generate` and related commands

Mirrors how `internal/ai/` is wired: a self-contained package loaded at entry
points.

### 4. Lazy re-encryption

Key versions are tracked per file. Bumping the current version (e.g., after a
member leaves) causes rela to re-encrypt on the next write. A manual rotation
command can force full re-encryption when needed.

## File Format

Per-value encryption uses a **key-prefix** marker — the encrypted property's
YAML key is prefixed with `_enc_v1_` and the value holds base64-encoded
ciphertext (nonce || ciphertext || AES-GCM tag):

```yaml
---
title: My Requirement              # cleartext
status: open                        # cleartext
_enc_v1_description: SGVsbG8gd29ybGQgY2lwaGVydGV4dA==
_enc_v1_notes: QW5vdGhlciBzZWNyZXQ=
_encryption:
    key_version: 1
    data_keys:
        engineering:
            jeroen: <hybrid wrapped data key (base64)>
            alice: <hybrid wrapped data key (base64)>
---

# My Requirement

(cleartext body — see encrypted_body below for the encrypted variant)
```

Why key-prefix, not a YAML `!enc` tag (earlier draft): `gopkg.in/yaml.v3`'s
`Unmarshaler` interface is not invoked when the destination is
`map[string]interface{}`, which is what fsstore's property loader uses. A YAML
tag would require a `*yaml.Node`-level parser migration. The key-prefix
approach is equivalent in expressiveness, needs no parser changes, and makes
encrypted files trivially greppable (`grep -r '_enc_v1_' entities/`).

The `_` underscore prefix on these keys is reserved: fsstore treats `_enc_v<N>_*`
and `_encryption` as internal metadata. Metamodel validation rejects user
property names starting with `_`.

An encrypted body uses a sibling frontmatter key `_encrypted_body` holding
base64 ciphertext; the markdown body section is left empty:

```yaml
---
id: TKT-123
type: ticket
title: My Requirement
_encrypted_body: QmFzZTY0ZW5jb2RlZENpcGhlcnRleHQ=
_encryption:
    key_version: 1
    data_keys: {...}
---
```

## Key Storage

- **Recipient public keys**: `keys/<identity>.pub` in the repo (PEM format), one file per team member. Filename identifies the recipient.
- **Groups config**: maps group names to lists of recipient filenames.
- **User private keys**: resolved in order — `$RELA_KEY_FILE`, `.rela/key` (project-local), `~/.config/rela/key` (user default). PEM format.

## What Stays Cleartext

- Entity IDs and filenames
- Relation files (both directions of every edge)
- Non-encrypted properties in the same entity
- All files in projects without encryption configured

Graph traversal, list filters on cleartext properties, and relation queries
continue to work without decryption.

## Known Limitations

- **Cache and graph**: `.rela/cache.json` and in-memory graph currently contain plaintext. A separate refactor is underway; cache/graph integration will be addressed once that lands.
- **Search on encrypted fields**: not supported (would require searchable encryption schemes — out of scope).
- **Merge conflicts in encrypted values**: conflicts appear as opaque ciphertext. Users must decrypt, resolve, and re-encrypt. SOPS-style deterministic output reduces spurious conflicts but does not eliminate them.
- **Departed members retain previous clones**: key rotation only protects *future* access. The content they had plaintext access to cannot be unshared. A full rotation + content review is still required after offboarding sensitive projects.

## Relationship to Other Systems

- **MCP server / data entry / desktop**: decrypt transparently when a private key is available. Without a key, encrypted values are unreadable but non-encrypted operations still function.
- **Lua scripts**: receive decrypted content when run locally with a key. Scripts running in contexts without a key (CI?) see only cleartext properties.
- **Git**: files remain plaintext-mergeable for cleartext fields; conflicts in `!enc` values must be resolved after decryption.
