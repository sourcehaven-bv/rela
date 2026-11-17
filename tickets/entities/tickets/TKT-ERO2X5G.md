---
id: TKT-ERO2X5G
type: ticket
title: Mutable record types
kind: enhancement
priority: medium
effort: m
started: "2026-11-16"
completed: "2026-11-17"
status: done
---

## Description

Record types are append-only. Some uses need rows that can be corrected in
place, such as company-wide hour registration. This adds an opt-in `mutable:
true` on a record type, so the operator decides per type.

## Scope

- `records.<type>.mutable: true` in schema.yaml. A ledger type cannot set it.
- Update one row: new fields (a null clears an optional field) and/or a new
`ts`. The owner of a row does not change.
- Delete one row.
- Who may update or delete: whoever may append, so update access on every
face of the owner. A hidden owner answers 404, as on read.
- Every row carries `updated_at`, `updated_by` and `revision`. An update or
delete names the revision it read; a stale one answers 409.
- Updates and deletes of mutable rows write an audit line
(`update-record`, `delete-record`). Appends stay out of the audit log.
- The store gains an optional `records.Mutator` capability; the file and
SQL backends implement it. A backend without it cannot serve a schema with a
mutable type.
- HTTP: `PATCH /api/v1/_records/{type}/{id}` and
`DELETE /api/v1/_records/{type}/{id}?revision=N`.

## Out of scope

- A general `closed_before` lock for non-ledger types.
- `readonly` fields.
- Per-row rights such as "only your own rows".
