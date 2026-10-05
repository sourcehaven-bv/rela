---
id: TKT-5W4ISW
type: ticket
title: Stores do not validate face grammar on create
kind: enhancement
priority: medium
effort: s
tags: security
status: backlog
description: Validate entity faces and relation tails with entity.ParseFace in every backend write path.
---

## Description

Deferred from TKT-KQXVF7 review (RR-O7YJGC). No store backend validates face
grammar on create:

- `CreateEntity` accepts any `entity.Face` string.
- `CreateRelation` accepts any `RelationKey.FromFace`; fsstore builds a
relation filename from it.

Face grammar is enforced at the boundary parsers (`entity.ParseRef`,
`entity.ParseFace`), and the Stage 2 address tests make malformed faces
`ErrNotFound` on reads. A write path that bypasses the parsers (import, sync,
data migration, Lua) could still persist a malformed face, and on fsstore a face
such as `../x` reaches a path.

## Approach

- Validate the face with `entity.ParseFace` in every backend's
`CreateEntity`, `ApplyEntity`, `CreateRelation` and relation update paths,
returning a typed invalid-input error.
- Add storetest cases for malformed faces on entity and relation writes (the
set the read-side address tests use: `../x`, `a@b`, NUL).

## Acceptance

- storetest rejects malformed entity faces and relation tails on all four
backends.
- fsstore never builds a path from an unvalidated face.
