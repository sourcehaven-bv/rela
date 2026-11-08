---
id: RR-6HJW5EY
type: review-response
title: memstore semantics differ from sqlite/postgres (no Tx rollback, no versions, no undeclared-type refusal)
finding: memstore/tx.go:11-21 mutual exclusion only; no VersionService on memstore; fsstore storeSchema refuses undeclared types.
severity: significant
resolution: Conformance claim removed; the guide lists the differences explicitly. AC2 narrowed to a single-write denial. Backend override stays in TKT-DAUS290.
status: addressed
---
