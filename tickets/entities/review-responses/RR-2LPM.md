---
id: RR-2LPM
type: review-response
title: DELETE-entity, PATCH-relation, DELETE-relation handlers skip the inaccessible guard
finding: 'internal/dataentry/api_v1.go: only handleV1UpdateEntity (line 527) checks `len(entity.Inaccessible) > 0`. handleV1DeleteEntity (line 591), handleV1UpdateRelation (line 768), handleV1DeleteRelation (line 810) all permit operations on inaccessible records. A confused or malicious SPA can DELETE the encrypted file (no key needed) — irrecoverable loss if not pushed yet. PATCH on encrypted relation rewrites it cleartext, destroying ciphertext. Same class of bug the entity PATCH guard solves. Fix: extract the guard into a shared helper (e.g. `requireWriteable(entity)`) and call from every write handler. For DELETE: decide policy explicitly — probably reject by default since the user typically cannot validate intent without first decrypting.'
severity: significant
status: deferred
reason: |-
    Parent ticket TKT-PGK91 (git-crypt detection) shipped via PR #668 without addressing this finding. Captured here so the gap remains visible; will be revisited if the underlying code path becomes a problem in practice. Closed as deferred via the TKT-5S8T data-debt sweep — the alternative is leaving the RR open indefinitely while it blocks every unrelated PR.
---
