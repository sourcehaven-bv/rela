---
id: TKT-TNERAS
type: ticket
title: 'tenant: erase an org — DROP SCHEMA CASCADE, a documented retention SLA, and the audit-log residue'
kind: enhancement
priority: medium
effort: m
tags: [security, needs-design]
status: backlog
---

## Description

A schema-per-tenant deployment gets a clean GDPR erasure story almost for free,
and RES-D54281 chose it partly for that. `DROP SCHEMA CASCADE` erases entities,
relations, **attachments** (bytes are in the database, `pgstore/attachment.go`),
search, and version history in one statement — because a schema holds *all* of a
tenant's data.

"Almost" free is doing work in that sentence. There are two residues outside the
schema boundary, and both must be decided rather than discovered.

## Residue 1: PITR backups

pgBackRest point-in-time-recovery backups retain a dropped schema for the whole
retention window. No amount of `DROP SCHEMA` changes that, and pretending
otherwise is the failure mode.

The standard, honest answer: **retention expiry *is* the erasure SLA.** Document
"erased within N days", not "immediately", and make N a number the operator can
state to a customer.

## Residue 2: the audit log

The audit sink is filesystem JSONL (`appbuild.go`), **shared across tenants** in
this design, and it records entity IDs, entity types, and principals. `DROP
SCHEMA` does not touch it.

This is **not a cross-tenant leak** — tenants have no disk access, and the log is
operator-oriented. But it **is retained personal data outside the erasure
boundary**, which is a different and still-real problem. The existing
`history-purge` tooling is per-entity and CLI-only, so it does not cover this.

Three options, and this ticket must pick one:

1. **A per-tenant sink** — the log follows the tenant and is deleted with it.
2. **A tenant field plus a retention policy** — one log, filterable, aged out.
3. **A documented exclusion** — the audit log is operator telemetry with its own
   retention, stated explicitly rather than left implicit.

`audit.Audit` is a one-method interface and the only hardcoding is a single
`filepath.Join`, so a per-tenant or DB-backed sink is a drop-in. The cost here is
the decision, not the code.

## Scope

**In scope**

- An erasure path that drops a tenant's schema and removes it from the tenant
  map, with the resident-set entry evicted and its store closed first (dropping
  a schema out from under an open pool is not a graceful shutdown).
- The retention SLA, written down where an operator will find it.
- The audit-log decision, implemented.

**Out of scope**

- Resolution (TKT-TNT9RS) and provisioning (TKT-TNPRV8).
- Per-entity GDPR purge, which already exists (`pgstore/purge.go`,
  `internal/cli/history_purge.go`) and is a different, row-level facility with
  no tenant dimension.

## Acceptance criteria

1. Erasing a tenant closes its store, evicts it from the resident set, drops its
   schema, and removes it from the map — in that order.
2. After erasure the org fails closed exactly as an unknown org does.
3. The backup-retention SLA is documented as a stated number of days.
4. The audit-log residue is resolved by an implemented decision, not a TODO.

## Notes

- Erasure is irreversible and reachable from an authenticated path, so it needs
  a sharper authorization story than "a verified principal from that org". Who
  may erase a tenant is part of the design work here.
