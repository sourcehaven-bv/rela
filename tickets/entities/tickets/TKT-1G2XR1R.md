---
id: TKT-1G2XR1R
type: ticket
title: Store content hash at write time and add a fingerprint column
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

The fingerprint of a range must be computable in SQL, so every live row needs
its hash at all times. Today pgstore fills `content_hash` lazily in the version
sweep (migration 0020), and it is NULL after every write.

- Compute `content_hash` at write time inside the store Tx (pgstore and sqlitestore). Keep the clearing trigger as the safety net, and keep the sweep working.
- Add a fingerprint column (defined in TKT-L0K1GHQ) for entities and relations, with an index suitable for range sums by address.
- sqlitestore gets the same columns through its migrations.
- fsstore and memstore compute both in memory.

## Acceptance

- No live row has a NULL hash after a write on any backend.
- Store conformance tests cover the hash and fingerprint after create, update, rename and delete.
