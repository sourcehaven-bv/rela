---
id: TKT-DOFYR1
type: ticket
title: Typed state references and the store contract (Step 1)
kind: enhancement
priority: high
effort: xl
status: ready
---

Design doc §2, §3, §6. The highest-engineering-risk piece: schema + contract
across all three backends, front-loaded so the compiler enforces the migration.

- `entity.Entity` gains `Pointer` (`""` = default state); relations gain a `from_pointer` tail; heads stay entity-level (§2.3).
- One boundary codec for the `PAGE-1@draft` serialization (URLs, CLI, acl.yaml, filenames); `entity.ValidateID` unchanged.
- Relation scope (`identity` | `content`) declared per relation type; default `identity`.
- Delete AND rename cascade over states + their relations — both backends currently exact-match (pg `entity.go:340`, fs `entity.go:282`).
- `EntityQuery` world scope, zero value = default world (79 non-test construction sites unchanged).
- Relation matching gains the pointer in BOTH implementations: `storeutil.MatchRelation` (fs/mem) and pgstore `relationWhere` SQL (`relation.go:298-335`) — v1's "single choke point" claim was false.
- `storetest` conformance cases for scope + cascade land BEFORE the second backend implements.
