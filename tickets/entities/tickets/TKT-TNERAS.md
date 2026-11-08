---
id: TKT-TNERAS
type: ticket
title: 'tenant: erase an org — DROP SCHEMA CASCADE, a documented retention SLA, and the audit-log residue'
kind: enhancement
priority: medium
effort: m
tags:
    - security
    - needs-design
started: "2026-11-08"
completed: "2026-11-08"
status: done
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

This is **not a cross-tenant leak** — tenants have no disk access, and the log
is operator-oriented. But it **is retained personal data outside the erasure
boundary**, which is a different and still-real problem. The existing
`history-purge` tooling is per-entity and CLI-only, so it does not cover this.

Three options, and this ticket must pick one:

1. **A per-tenant sink** — the log follows the tenant and is deleted with it.
2. **A tenant field plus a retention policy** — one log, filterable, aged out.
3. **A documented exclusion** — the audit log is operator telemetry with its own
retention, stated explicitly rather than left implicit.

`audit.Audit` is a one-method interface and the only hardcoding is a single
`filepath.Join`, so a per-tenant or DB-backed sink is a drop-in. The cost here
is the decision, not the code.

## Scope

**In scope**

- An erasure path that drops a tenant's schema and removes it from the tenant
map, with the resident-set entry evicted and its store closed first (dropping a
schema out from under an open pool is not a graceful shutdown).
- The retention SLA, written down where an operator will find it.
- The audit-log decision, implemented.

**Out of scope**

- Resolution (TKT-TNT9RS) and provisioning (TKT-TNPRV8).
- Per-entity GDPR purge, which already exists (`pgstore/purge.go`,
`internal/cli/history_purge.go`) and is a different, row-level facility with no
tenant dimension.

## Acceptance criteria

Revised during implementation (in TKT-D6MF3SW's PR). The erasure runs as an
operator command in its own process, not inside a running server, so it cannot
evict a resident tenant itself; the order is enforced across processes instead.

1. A tenant marked `erase: true` in `tenants.yaml` is not served: its org
fails closed exactly as an unknown org does, and its schema name stays reserved.
(`TestConfig_MarkedTenantIsNotServed`,
`TestConfig_MarkedTenantStillReservesItsSchema`)
2. `rela-server tenant erase ORG_ID` refuses an unknown org, an unmarked
org, an invalid or shared schema name and an unconfirmed run.
(`TestEraseTenant_Refusals`, `TestErase_RefusesBadInput`,
`TestErase_RefusesSharedSchemas`)
3. Erasure is refused while any server has the tenant open, and no server
can open the tenant while it runs (schema advisory lock).
(`TestErase_RemovesOnlyThatTenant`, `TestErase_RefusesOpenDuringErasure`)
4. Erasing tenant A drops A's schema and removes A's audit directory, and
leaves tenant B's schema, rows and audit records.
(`TestErase_RemovesOnlyThatTenant`, `TestEraseTenant_EndToEnd`)
5. If the lock was lost and a server still has the tenant open, its queries
fail and never fall back to other data. (`TestErase_LostLockFailsClosed`,
`TestErase_RefusesWhenPublicHasTables`)
6. The backup-retention guarantee and the audit-log decision are documented
(GUIDE-postgres-backend, "Erasing a tenant").

## Decisions

- **Audit log:** option 1, a per-tenant sink (TKT-SNVR8UV), deleted with the
tenant. A host-level `tenant-erasures.jsonl` records each erasure without tenant
data.
- **Who may erase:** an operator at the host, who can read `tenants.yaml`
and reach the database, after marking the tenant and restarting the servers.
Never the IdP webhook: `organization.deleted` only logs an erasure-needed
record.
- **Retention:** erasure is complete when the deployment's PITR retention
period has passed; the operator states that period in days.
- **Audit retention:** erasing the tenant's audit log overrides the 3-year
audit retention, because it is that customer's data. The operator first checks
that no open incident or nonconformity references the tenant.
- **Architecture:** `cmd/rela-server/tenanterase.go` links mostly to `tenant`,
because it is the operator command edge and the erasure lives in `tenant`.
The archplan finding is accepted and the ratchet loosened with that reason.
The command finds the cache directory through `appbuild.DiscoverCacheDir`
rather than importing `project` and `storage` itself: the extra users of those
files shifted archplan's damped link weights enough to report an unrelated
file (`internal/cli/classification.go`) as misplaced.

## Resolution

Implemented in the TKT-D6MF3SW pull request (branch
`tkt-d6mf3sw-multitenant-host`): `internal/tenant/erase.go`,
`internal/tenant/erase_postgres.go`, `cmd/rela-server/tenanterase.go`.
