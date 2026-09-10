---
id: RR-WSWB8P
type: review-response
title: Store type-mismatch probe was gated on a non-default face
finding: The store's type-mismatch probe only ran when the written face was non-default, so a write to the zero coordinate could land a second entity type under an existing id. sqlitestore and pgstore both exhibited it; fsstore and memstore refused it incidentally rather than by rule.
severity: critical
resolution: Removed the !e.Face.IsDefault() gate so the probe runs on every write, using WHERE id = $1 FOR SHARE in pgstore. Added TypeMismatchRejectedOnTheZeroCoordinate to the shared storetest conformance suite so all four backends are held to it.
status: addressed
---

## Finding

The type-mismatch probe was gated on `!e.Face.IsDefault()`. A write to the zero
coordinate therefore skipped it, allowing a second entity type to be stored
under an existing id.

`sqlitestore` and `pgstore` both exhibited this. `fsstore` and `memstore`
refused it, but incidentally (through path/key layout) rather than by rule,
which is exactly the kind of divergence the conformance suite exists to catch.

## Resolution

Removed the gate so the probe runs on every write; `pgstore` takes `WHERE id =
$1 FOR SHARE`.

Added `TypeMismatchRejectedOnTheZeroCoordinate` to `internal/store/storetest` so
the rule is enforced against all four backends rather than assumed.
