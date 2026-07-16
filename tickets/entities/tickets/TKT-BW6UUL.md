---
id: TKT-BW6UUL
type: ticket
title: 'Operator ''purge version'' primitive: hard-delete a snapshot row for compliance redaction'
kind: enhancement
priority: low
effort: s
status: done
---

Follow-up to TKT-9INY0Y (entity versioning) and TKT-92JL8P (relation
versioning); design-review finding RR-A3RNT0. **Design revised after
design-review (cranky + go-architect) — 3 critical + 4 significant findings
(RR-SH28E, RR-EQQP1, RR-17DMC, RR-TXV38, RR-YVVUC, RR-ECUWV, RR-NGFYE,
RR-8FUVW). IMPLEMENTED + DB-verified + CI-green on
feat/version-purge-TKT-BW6UUL.**

## What it does

Operator-only **purge** that hard-deletes version snapshot rows from
`entity_versions` / `relation_versions` (entities + relations) for compliance
redaction — the deliberate, audited, IRREVERSIBLE exception to append-only
history. CLI: `history-purge` / `relation-history-purge`, dry-run by default.

## Design (all review findings implemented)

1. **Sweep re-capture guard (RR-SH28E):** purge runs under `sweepAdvisoryLockKey`
(mutually exclusive with a sweep tick) and REFUSES when a live row still holds
the content unless `--force-live`, which writes a no-content `VersionOpPurge`
tombstone (content_hash = live hash) that the sweep's existing dedup respects,
so it never re-captures the purged content.
2. **Rename-row guard (RR-EQQP1):** v1 refuses to purge a `rename` row (purging
one orphans/forks the lineage walk); non-rename only.
3. **Fenced --all (RR-ECUWV):** `--all` purges the fenced lineage (`lineageCTE` /
`relationLineageIDs`), never `WHERE id=$1` (id-reuse safety).
4. **Auth = operator shell (RR-17DMC):** v1 CLI trust boundary is shell +
RELA_DATABASE_URL (no ACL check, like `db migrate`); `PermHistoryPurge` left for
a future API surface, not wired into the CLI.
5. **Audit via the sink (RR-TXV38):** `OpPurgeVersion` through `audit.Audit`
(`svc.Audit()` on cliServices), not a Manager method; records identity + vseq
range + count + `--reason` + principal, NEVER the purged content.
6. **One method per capability (RR-YVVUC):** separate `VersionPurger` /
`RelationVersionPurger`, one `PurgeVersions`/`PurgeRelationVersions` each;
plimsoll 37→39. Stable `--vseq`/`--content-hash` selectors (not the ordinal).
7. **UX (RR-NGFYE, RR-8FUVW):** dry-run default + `--commit`; type-the-id
confirmation (`--yes` for scripts); `--content-hash` = verifiable
erase-everywhere; distinct "nothing to purge".
8. **Completeness (RR-NGFYE):** docs state purge is necessary-not-sufficient
(live row / PITR backups survive); `schema_versions` is projection-only +
FK-shared, never deleted by purge.

## Store capability

`purge.go`: `PurgeVersions` / `PurgeRelationVersions` under the advisory lock;
`PurgeSelector` (Vseq | ContentHash | All); `PurgeResult` (targets, count,
LiveRowExists, RenameInTargets, TombstoneWritten); DryRun resolves without
deleting. 8 DB-gated tests incl. the RR-SH28E end-to-end sweep-suppression
proof.

## Out of scope (v2 / separate tickets)

Data-entry/API purge surface + `PermHistoryPurge` enforcement; rename-row purge
with lineage-severance handling; automatic age/count retention; cascading purge.

## Origin

Deferred from TKT-9INY0Y (RR-A3RNT0); re-scoped for relations in the TKT-92JL8P
follow-up; hardened by the design review (the sweep-recapture and rename-orphan
findings would each have shipped a false compliance guarantee).
