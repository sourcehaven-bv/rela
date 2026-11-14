---
id: TKT-14J8NJO
type: ticket
title: Reject id prefixes shared across entity types
kind: chore
priority: low
effort: s
status: backlog
description: The loader accepts two entity types whose id prefixes are equal or differ only in case, although ids are case-insensitive identities and such types share one id space and one sequential counter.
---

## Description

`metamodel` validates each `id_prefix` on its own (`loader.go`, around line
372). Two types declaring `REQ-`, or `nt-` and `NT-`, share one id space: the
stores fold id case, both types number from one counter, and each type's prefix
check accepts the other's ids. Since BUG-4OPP525 `Metamodel.InferEntityType`
resolves such overlaps deterministically, but the schema is still ambiguous.

## Expected

Decide whether a prefix shared across types is ever intended. If not, reject it
at load with an error naming both types, compared with case folded, and add a
loader test. Check external schemas first, because this refuses a schema that
loads today.
