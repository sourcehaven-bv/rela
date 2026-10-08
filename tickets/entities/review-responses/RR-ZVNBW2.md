---
id: RR-ZVNBW2
type: review-response
title: Sealed conformance only over FSKV
finding: Not run over sqlite or pg state KV, the only tiers where Sealed runs.
severity: minor
resolution: Conformance suite runs over the sqlite state store and the pg state store via a schema-pinned DSN.
status: addressed
---
