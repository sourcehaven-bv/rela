---
id: DEC-D5P4X
type: decision
title: Use age for repo-level file encryption instead of a custom envelope
context: 'A prior attempt at per-property encryption had ship-blocker bugs (tamper collapsed to ''no matching key'', key_version not validated) that shipped because production wiring differed from test fakes. The simpler per-repo design shrinks the problem, but still required a custom envelope: magic header, version byte, recipient wraps, per-file data key, AEAD body, fingerprint. Design review pointed out that every one of those properties is already solved by age (filippo.io/age).'
consequences: |-
    - We import filippo.io/age and build the leaf package as a thin facade. The ~600 LoC custom wrap.go + aead.go + envelope spec + version byte + magic bytes collapse into age.Encrypt/age.Decrypt calls.
    - Users get `age -d sealed-file.md` as a recovery escape hatch when rela is broken or being audited.
    - Post-quantum hybrid (X25519 + ML-KEM-768) becomes an age recipient plugin, deferred to a follow-up ticket. v1 ships with X25519 only (age's built-in). Consequence: v1 is not post-quantum; revisit when the PQ recipient plugin lands.
    - Wire format becomes 'age-encryption.org/v1' instead of our own 'RLAE'. Third-party tooling (sops, age-plugin-yubikey) composes.
    - Audit story improves: 'we use age' vs. 'we wrote a new envelope'.
    - The keys directory convention (<repo>/keys/<name>.pub) becomes a thin wrapper that stores age public keys in age's ASCII format.
    - The recipient-set fingerprint is dropped (per design review S-2: lint theater without a signing mechanism). Replaced with a startup-time warning when the committed recipient list diverges from the loaded keyring.
date: "2026-04-19"
status: accepted
---
