---
id: TKT-1A2KE3
type: ticket
title: entitymanager relation API takes entity.RelationKey
kind: refactor
priority: medium
effort: m
tags: tech-debt
status: backlog
description: Replace RelationOptions.FromFace and DeleteRelationState with RelationKey on the entitymanager relation methods.
---

## Description

Stage 2 (TKT-KQXVF7) moved the store relation API onto `entity.RelationKey`. The
`entitymanager` relation API did not follow:

- `Manager.CreateRelation` and `Manager.UpdateRelation` take
`(from, relType, to string, opts entity.RelationOptions)` and carry the tail in
`RelationOptions.FromFace`.
- `Manager.DeleteRelation(from, relType, to)` addresses the default tail;
`Manager.DeleteRelationState(from, face, relType, to)` is the general form.

So a caller can still drop the tail by omission, which is the mistake the store
flip and the `tailless` archguard exist to prevent.

## Approach

- `CreateRelation`, `UpdateRelation` and `DeleteRelation` take an
`entity.RelationKey`; remove `RelationOptions.FromFace` and
`DeleteRelationState`.
- Update callers in `dataentry`, `mcp`, `cli`, `importer`, `docs`,
`appbuild`, `aclmap`, the Lua bindings and `entitymanagertest`.
- Extend the `tailless` archguard scan to the new call sites.

Related: TKT-0VJ0HV (single-relation data-entry routes are default-tail only).

## Acceptance

- No `entitymanager` relation method takes a separate face argument or a
`FromFace` option.
- Existing faced relation tests pass with unchanged behaviour.
