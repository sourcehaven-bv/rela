---
id: TKT-QMF1GU
type: ticket
title: Audit records carry the face they act on
kind: enhancement
priority: medium
effort: m
status: backlog
description: audit.Subject has no face field, so entity and relation write audits on a faced type do not say which face changed.
---

## Description

`audit.Subject` has no face field. Entity and relation write audits on a faced
type therefore do not record which face (or which relation tail) they changed.
The history purge audit names the face in its summary only (RR-SGJVSA, deferred
from BUG-4SYAA6).

## Scope

- Add the face (entity) and the tail face (relation) to `audit.Subject`.
- Stamp it on every write operation that addresses a face: create, update,
patch, delete of one face, relation writes on a content-scoped tail, restore and
purge.
- Move the purge summary face into the new field.
- Keep the JSONL format backward compatible: the field is omitted for faceless
types.
