---
id: IMPL-M7RLC8
type: implementation-checklist
title: 'Implementation: SQLite content versioning: entity + relation history, sweep, and purge behind the sqlite build tag'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

The implementation is four new files in `sqlitestore` (`version.go`,
`relation_version.go`, `sweep.go`, `purge.go`), the versioning DDL as a
migration rung in `sqlitedb`, and a shared conformance suite in `storetest`.

Two things the plan did not anticipate, both forced by TKT-S1EVV7 landing the
connection split first:

1. The schema ladder now lives in `internal/sqlitedb`, so the versioning DDL is
   rung **v3→v4** rather than a private v1→v2 step, and the planned
   `sqlitestore/migrate.go` was dropped entirely.
2. `ALTER TABLE relations ADD COLUMN rel_record_id` cannot sit in the shared
   DDL (no `IF NOT EXISTS` form, and `schemaSQL` re-runs on every open while
   also indexing that column). Fresh databases declare the column inline in
   `CREATE TABLE relations`; existing ones get it from a pragma-probed
   `ensureRelRecordIDColumn` that runs ahead of `schemaSQL`.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The conformance suite takes a `storetest.Factory`, so every case builds its own
store and the backend under test is a parameter rather than an assumption.
`writeVersion` fills the fields every backend needs so individual cases stay
about the behaviour they name.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Evidence |
|----|----------|
| 1 — seven capabilities wired | `TestSQLiteStoreSatisfiesTheVersionCapabilities` + `TestSQLiteVersionServiceIsUsable` (appbuild, sqlite tag). Asks the RESOLVER, not the interface — see AC-1 note below. |
| 2 — shared harness green on both | `storetest.RunVersionTests`: 31 cases. sqlite `go test -tags sqlite ./internal/store/sqlitestore/` PASS; pgstore against a real PostgreSQL 17 PASS, **0 skips**. |
| 3 — per-face history | `Version/Faces` (3 cases): independent lineages, `ListVersions` is the default face, identical bytes across faces stay distinct. |
| 4 — lineage fencing | `Version/Lineage`: rename stitches A→B→C; a reused id does not merge histories. Relation side: delete+recreate mints a fresh `rel_record_id`. |
| 5 — purge guardrails | `Version/Purge` (9 cases): dry-run default, refuses a rename row, refuses while a live row holds the content, `--force-live` tombstones, selector required, multi-lifetime refused. |
| 6 — migration | `TestMigrateToVersioningBackfillsDistinctLineageIDs`, `TestMigrateToVersioningIsIdempotent`, `TestFreshDatabaseHasTheVersioningSchema`; the pre-existing `TestRefusesNewerSchema` still passes. |
| 7 — history reaches the UI | `TestHistoryIsServedOnSQLite` drives the real HTTP handler over a real sqlitestore: HTTP 200 with the captured snapshot content, not 501. `TestConfigHistoryEnabledOnSQLite` pins the SPA flag. |
| 8 — backend isolation | `go list -deps` checked locally for all four combinations; `sqlite,postgres` and `sqlite,memorybackend` still refuse to build. |
| 9 — attribution | `Version/EntityHistory/AttributionRoundTrips`; sweep rows carry `version-sweep` (`sweepPrincipalTool`). See the known limitation below. |

**AC-1 is the one worth reading twice.** The seven capabilities were fully
implemented and their conformance suite passed while the feature was still
completely unreachable from a running app: `versionServiceFor` lived behind
`//go:build !postgres` and returned nil, so every history request on the sqlite
build got 501 and the SPA hid the History button. A compile-time interface
assertion would not have caught it — the store satisfied the interfaces the
whole time. The tests therefore ask the resolver and the HTTP handler, which
are the things that were actually wrong.

**Known limitation (AC-9, deliberate).** sqlitestore's live rows carry no
`last_edited_by_user/_tool` columns, so **sweep-captured** create/update
versions always take the `version-sweep` system principal rather than copying
the editing principal across as pgstore does. Synchronous captures
(rename/delete) carry the real principal end to end. This satisfies AC-9's
actual requirement — never a guessed or literal-"unknown" identity — and the
editing principal stays recoverable from the audit log. Closing the gap means
adding attribution columns to `entities`/`relations`, which is a schema change
to the live tables and out of scope here; it is documented at the fallback in
`sweep.go`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
      patterns extracted to a helper / constant / type where it
      sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Three extractions earned their place, and each removed a real duplicate rather
than anticipating one:

- `versionSchemaSQL` is shared between the fresh-database path and the
  migration rung, exactly as `projectFilesDDL` and `stateKVDDL` already are —
  the pattern this file was told to follow.
- `versionsweep_shared.go` holds the resolvers both database backends use.
  They were postgres-only files whose contents named no postgres type, so
  copying them for sqlite would have created two copies of a
  typed-nil guard that had already been got wrong once (TKT-L3FNEN).
- `storetest.RunVersionTests` replaces what would otherwise be sqlite-local
  tests duplicating pgstore-local ones.

`nonNilCapability` lost its `//go:build postgres` tag and moved to
`capability.go`, because the sqlite resolver needs the same typed-nil guard.
