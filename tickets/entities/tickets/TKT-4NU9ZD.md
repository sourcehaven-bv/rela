---
id: TKT-4NU9ZD
type: ticket
title: 'SQLite content versioning: entity + relation history, sweep, and purge behind the sqlite build tag'
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Implement the content-versioning optional capabilities for `sqlitestore`,
closing the last substantive gap between the SQLite and PostgreSQL backends.
This is stage 4 of DEC-LFSYNY, left **unestimated until scoped** by TKT-TWIO11
AC-6.

In scope, all seven capabilities plus the sweep:

- `store.HistoryReader` / `store.VersionWriter` (entity)
- `store.StateHistoryReader` (per-**face** history — see design note 0)
- `store.RelationHistoryReader` / `store.RelationVersionWriter`
- `store.VersionPurger` / `store.RelationVersionPurger`
- `store.VersionSweeper` (debounced create/update reconciliation)

## Why

Versioning is currently absent on sqlite. It degrades honestly rather than
misbehaving — `internal/dataentry/history_handler.go:45` returns HTTP 501
`history_unsupported` with a plain message, because a capability gap is not an
ACL decision — so this is a **feature gap, not a defect**. Nothing is broken;
history is simply unavailable on a backend where users will reasonably expect
it, since the whole point of SQLite in rela's lineup is "pgstore's properties
without pgstore's operational cost" and versioning is one of those properties.

fsstore gets versioning free from git. pgstore has it. SQLite, which targets
single-server and desktop deployments, has neither — and unlike fsstore it
cannot fall back on git, because the markdown files are not the source of truth.

## Prerequisite work: already done

The interface-level blockers are cleared, which is what makes this a bounded
port rather than a redesign:

- **TKT-415WA7** widened pgstore's capability assertions to interfaces.
- **TKT-L3FNEN** promoted `SweepConfig`, `ProjectionProvider`, `VersionService`
and `VersionSweeper` into `internal/store`, so a second backend can satisfy them
without importing pgstore. `backendneutral_postgres_test.go` pins that (AC-2):
it satisfies every capability using store-package types only.

Consumer-side wiring is therefore already backend-neutral. The single seam to
fill is `internal/appbuild/versionsweep_nosweep.go`, where `versionServiceFor`
returns nil for every non-postgres build.

*(Implementation note: that seam was larger than one line. `versionServiceFor`
and `stateKVFor` shared one `//go:build !postgres` file, so versioning could
not be enabled for sqlite without also enabling the database-backed state KV
this ticket puts out of scope. The file was split by concern instead —
`versionsweep_shared.go` for the backend-neutral resolvers,
`statekv_nodb.go` for the FSKV fallback.)*

## Design notes (established during scoping, verify in planning)

**0. Versioning is per-FACE, not per-entity (rescoped 2026-09-04).** The worlds
/ content-states work (FEAT-9CD2MX, #1452, commit `e0187047`) landed after this
ticket was first written and changed the target underneath it. `VersionService`
gained a seventh member, `StateHistoryReader`, whose methods take an
`entity.Face` coordinate. The umbrella's own doc states the rule to inherit: a
backend that versions entities at all versions their faces — "the face is a
coordinate on the row, not a separate capability to negotiate." So there is no
option to ship entity versioning first and faces later.

The mitigating fact: `sqlitestore` is **already face-aware**. Faces are carried
in its schema, `entityquery.go` filters on them, `relation.go` threads them
through relation keys, and `observer.go` emits per-face delete notifications.
The port therefore lands on a store that already models the coordinate; what is
missing is the version tables and the read/write/sweep/purge logic over them.

**1. The advisory lock has no sqlite analogue, and needs none.** pgstore's sweep
runs its entire tick on one acquired connection under `pg_try_advisory_lock`,
because several `rela-server` processes may share a database. `sqlitestore.Open`
takes an **exclusive sidecar lock** and refuses a second process (`lock.go` —
load-bearing, because `unique:` is an untransacted scan with no backstop). So
"only one sweeper" is guaranteed by construction. Do not port the advisory-lock
machinery; do document why it is absent, or the next reader will assume it was
forgotten.

**2. Sequence allocation.** pgstore uses a dedicated `version_seq`, deliberately
NOT `rela_seq`, so version rows do not erode the change-feed watermark's overlap
budget. SQLite has no sequences; decide between an `AUTOINCREMENT` column and a
counter table, and preserve the same separation from any change-feed sequence.

**3. Schema migration.** *(Superseded during implementation.)* The note below
described a ladder local to `sqlitestore` at `user_version` 1. TKT-S1EVV7 has
since moved the database file — opening, PRAGMAs, the single-writer lock and a
real migration ladder — into `internal/sqlitedb`, which was already at v3. The
versioning schema is therefore rung **v3→v4 there**, and the planned
`sqlitestore/migrate.go` was dropped. The requirement is unchanged: a real
migration step with its own test, not a bumped constant.

Original note: `sqlitestore` is forward-only via `PRAGMA user_version`
(currently 1; `version_test.go` pins both the stamp and the refusal of a newer
database). Versioning tables mean `user_version = 2` plus a real migration step
— the first time that path is exercised, so it needs its own test, not just a
bumped constant.

**4. Port surface.** pgstore's implementation is ~1,960 non-test lines
(`version.go` 380, `relation_version.go` 533, `sweep.go` 584, `purge.go` 465) —
re-measured 2026-09-04 after the worlds work grew all four files, up from ~1,730.
Relevant migrations are now `0004_versions.sql`, `0005_relation_versions.sql`
and `0012_per_state_versions.sql`, and `states_versioning_test.go` (250 lines)
is the per-face behaviour to match. Expect meaningfully less for sqlite (no advisory
locking, no schema-per-tenant, no `LISTEN/NOTIFY` interaction), but the lineage
logic is irreducible.

**5. Lineage fencing is the subtle part.** Entity lineage uses `[lo,hi)` vseq
ranges in a recursive CTE — an unbounded `entity_id = ANY(...)` read merges two
entities' histories after an id reuse. Relations key lineage on a surrogate
`rel_record_id` column ON the relations row (not reconstructed per tick, which
would race the sync path). SQLite supports recursive CTEs, so the approach
ports; the correctness argument must be re-derived, not assumed.

**6. Purge guardrails are load-bearing.** Mutual exclusion with a sweep tick,
refusal while a live row still holds the content (unless `--force-live`),
refusal of a rename row, and `--all` purging the fenced lineage rather than
`WHERE id=$1`. These are design-review outcomes; carry every one across.

## Out of scope

- **State KV.** Deliberately closed, not deferred: `versionsweep_nosweep.go`
documents that sqlite inherits the filesystem KV on purpose (TKT-L1A3PH),
because node-local state is only a problem across processes and sqlitestore is
single-process by construction. "Node-local" and "the only node" coincide.
- Multi-process SQLite coordination (rejected in TKT-TWIO11 AC-4).
- Backend-to-backend migration of existing history.

## Acceptance criteria

1. All seven versioning capabilities implemented on `sqlitestore` and type-asserted
at the wiring site; `versionServiceFor` returns a working service on the sqlite
build.
2. A **shared conformance harness** covers versioning for any backend that
claims it, rather than sqlite-local tests duplicating pgstore-local ones.
`internal/store/storetest` has no versioning suite today, so pgstore's
guarantees are pinned only inside pgstore — a second implementation is exactly
the trigger for extracting one, and it must then run green against pgstore too.
Follows the precedent of `Capabilities{TxRollback}` from TKT-8TJ2WN: the backend
DECLARES the tier and the suite runs, so a backend that has the capability and
forgets to say so gets no silent pass.
3. Per-face history works for a multi-face entity, matching the behaviour
`pgstore/states_versioning_test.go` pins — not merely compiling against the
`entity.Face` signature.
4. Lineage fencing verified across rename and id-reuse for both entities and
relations, with a test that fails if the fence is removed.
5. Purge guardrails all present, with the audit record and dry-run default.
6. `user_version` migration tested (landed as the sqlitedb ladder's v3→v4 rung
rather than a sqlitestore-local 1→2 step — see design note 3), including refusal
of a newer database.
7. History reaches the data-entry UI: the 501 `history_unsupported` path is no
longer taken on the sqlite build, verified end-to-end rather than by unit
assertion.
8. Backend isolation still holds — the default build links no
`modernc.org/sqlite`, the sqlite build no `pgx`.
9. Attribution matches pgstore: `store.WithAttribution` at the entitymanager
boundary only, never a guessed or literal-"unknown" principal; NULL columns fall
back to the `version-sweep` system principal.
