---
id: repo-encryption
type: concept
title: Repository-Level At-Rest Encryption
summary: 'Whole-file sealing of entity and relation markdown using a single repo-level data key, wrapped for each recipient via hybrid X25519 + ML-KEM-768. No per-property or per-group complexity: the metamodel stays cleartext, but every file under entities/ and relations/ is sealed as an atomic blob. Adding a recipient requires only rewrapping the data key; removing one requires rewrapping AND generating a new data key (which implies re-encrypting all files). The envelope advertises the recipient-set fingerprint so the tool can refuse to write when local keyring membership diverges — a tamper/awareness signal, not a cryptographic enforcement of past reads.'
description: |-
    Design goals:
    - **Coverage**: 90%+ of use cases where the project wants its contents private; no mixed-sensitivity data-leak edge cases.
    - **Simplicity**: one data key, one recipient list, one envelope format.
    - **Boundary**: sealing happens at the fsstore I/O boundary — read a file, check for magic header, unseal; marshal for write, then seal. No frontmatter-vs-body split, no property-level syntax.
    - **Metamodel stays cleartext**: schemas and groups.yaml are not secrets; tooling needs them to bootstrap.

    Threat model:
    - Protects against: casual disk read, accidental git push to a public remote, stolen laptop without the private key.
    - Does NOT protect against: recipients who decrypted past files keeping copies (cryptography cannot enforce forgetting).

    Key rotation semantics:
    - **Add member**: rewrap existing data key under new recipient's public key. No file rewrites needed.
    - **Remove member**: generate fresh data key, decrypt every file with old key, re-encrypt with new key. Recipient-set fingerprint in the envelope flips, so old-envelope-in-current-working-tree becomes detectable.

    What this replaces: the earlier per-group/per-property design (metamodel `encrypted:` declarations, groups.yaml, _enc_v1_ property prefix, _encryption YAML block). That design is superseded because it (a) created within-file leakage bugs requiring per-group data keys to fix, (b) demanded a keyring wiring layer per property group, (c) couldn't protect whole-body content with the same confidence as whole-file sealing.
package: internal/encryption
layer: infra
status: draft
---
