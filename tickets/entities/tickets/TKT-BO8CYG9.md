---
id: TKT-BO8CYG9
type: ticket
title: POST /api/v1/_batch endpoint, OpenAPI and API reference
kind: enhancement
priority: medium
effort: m
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

## Description

Add `POST /api/v1/_batch` on top of `entitymanager.CreateBatch` (DEC-ODU65KR).
The body lists `entities` (each with an optional `ref` label) and `relations`
whose endpoints are `{"ref": ...}` for an entity in the batch or `{"id": ...,
"face": ...}` for an existing one.

- 501 `atomic_batch_unsupported` when the store cannot roll back.
- Fixed limits: 100 operations, 1 MiB body (413 `body_too_large`).
- 400 `batch_invalid` with a JSON pointer for a duplicate or unknown ref, or
a ref used before it is defined; strict decode.
- Errors are problem+json with `operation {kind, index, ref}`.
- 200 lists the created entities by ref with their ids, each redacted for
the caller, plus warnings and follow-up errors per operation.
- OpenAPI entry, and the docs-project source of
`docs/data-entry/api-reference.md`: replace the stale "no transaction primitive"
text and the "out of scope" line.
