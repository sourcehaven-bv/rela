---
id: PLAN-4UUWH6
type: planning-checklist
title: Planning
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: all seven `store.VersionService` capabilities on `sqlitestore`
(`HistoryReader`, `StateHistoryReader`, `VersionWriter`,
`RelationHistoryReader`, `RelationVersionWriter`, `VersionPurger`,
`RelationVersionPurger`) plus `VersionSweeper`; a shared versioning conformance
harness in `storetest` that also runs green against pgstore; the `user_version`
1→2 migration.

OUT: state KV (deliberately closed — sqlite is single-process, so the FSKV
fallback is correct, per TKT-L1A3PH); multi-process coordination (rejected in
TKT-TWIO11 AC-4); backend-to-backend history migration.

**Acceptance Criteria:** the nine on TKT-4NU9ZD; each mapped to a test under
Test Plan below.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: RES-03TUXO already surveyed
the SQLite backend; this ticket ports a design that already exists in-tree, so
there are no options to survey — the reference implementation IS the spec)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — see above. RES-03TUXO covers the backend decision.

**Existing Solutions:**

The reference implementation is `internal/store/pgstore`, ~1,962 non-test lines
across `version.go` (380), `relation_version.go` (533), `sweep.go` (584),
`purge.go` (465), plus migrations `0004_versions.sql`,
`0005_relation_versions.sql`, `0012_per_state_versions.sql`,
`0013_write_origin.sql`. Those migration files are unusually well-commented and
function as the design document — each explains not just what it creates but
which bug the shape prevents.

No external library is a candidate: this is domain logic over the store's own
schema, not a reusable versioning engine.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Port the pgstore design, translating four backend-specific mechanisms:

1. **Sequences → a counter.** pg uses `version_seq` and `relation_record_seq`,
both deliberately separate from `rela_seq` so version rows never erode the
change-feed watermark's overlap budget. SQLite has no sequences. Use `INTEGER
PRIMARY KEY AUTOINCREMENT` for `vseq` monotonicity, keeping the same separation
from any change-feed sequence.

2. **Advisory lock → nothing.** pg's sweep runs its whole tick on one connection
under `pg_try_advisory_lock` because several processes may share a database.
`sqlitestore.Open` takes an exclusive sidecar lock and refuses a second process
(`lock.go`), so single-sweeper is guaranteed by construction. Do not port the
lock; DO document its absence, or the next reader assumes it was forgotten.

3. **JSONB → TEXT.** Properties are stored as JSON text, matching how
`sqlitestore/jsonprops.go` already handles the entities table.

4. **`ON CONFLICT DO NOTHING`, recursive CTEs, partial indexes** all port
verbatim — SQLite supports each.

**The fenced-lineage CTE ports unchanged — verified empirically, not assumed.**
I ran pgstore's recursive lineage CTE against `modernc.org/sqlite` with an A→B→C
rename chain plus a reused `A` id, and it returned exactly `[A B C]` with the
reused id correctly fenced out. This was the single biggest portability risk; it
is retired with evidence.

Alternatives rejected:
- *A generic cross-backend versioning layer over `store.Store`.* Rejected: it
cannot express the per-backend fencing (pg's `[lo,hi)` vseq windows) without
re-implementing the store, and CLAUDE.md forbids repository abstractions.
- *Ship entity versioning first, faces later.* Rejected — not available:
`VersionService`'s own doc states "the face is a coordinate on the row, not a
separate capability to negotiate."

**Files to modify:**

- `internal/store/sqlitestore/version.go` (new) — entity + state history, writer
- `internal/store/sqlitestore/relation_version.go` (new)
- `internal/store/sqlitestore/sweep.go` (new)
- `internal/store/sqlitestore/purge.go` (new)
- `internal/store/sqlitestore/sqlitestore.go` — schema v2, `VersionStore()`
- `internal/store/storetest/version.go` (new) — the shared harness
- `internal/store/storetest/storetest.go` — `Capabilities{Versioning}`
- `internal/appbuild/versionsweep_sqlite.go` (new) + the `!postgres` seam

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Entity ids, faces and relation triples reach the store already validated
(`isSafePathSegment`, `ValidateRelationType`, the `entity.Face` grammar). Every
query is parameterized — no string interpolation into SQL, matching the rest of
sqlitestore. Version content is entity content: already-trusted-at-rest bytes
that are re-sanitized on render, not here.

**Security-Sensitive Operations:**

1. **Purge is irreversible hard-deletion.** Guardrails are load-bearing design
review outcomes and every one must be carried across: mutual exclusion with a
sweep tick; refusal while a live row still holds the content (unless
`--force-live`, else the sweep re-captures within one interval); refusal of a
rename row (purging one orphans the lineage walk); `--all` purges the fenced
lineage, never `WHERE id=$1` (id-reuse would destroy unrelated history). Trust
boundary is the operator shell; audited via `audit.Audit`, never echoing purged
content.
2. **Attribution must not be guessed.** `store.WithAttribution` is set only at
the entitymanager boundary and only for a real principal; NULL columns fall back
to the `version-sweep` system principal, never a literal "unknown".
3. **Read gating is the caller's job.** Relation history is gated on BOTH
endpoints (FROM ∧ TO) upstream — the FROM entity owns UI placement, not the auth
boundary. The store stays ungated; visibility wrappers gate. Relations have no
field-level redaction, so relation history exposes exactly what a live relation
GET does.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

The centrepiece is AC-2: a **shared** harness in `storetest`, gated by a
declared `Capabilities{Versioning}` flag, following the `TxRollback` precedent
from TKT-8TJ2WN — the backend declares the tier and the suite runs, so a backend
that has the capability and forgets to say so gets no silent pass. It must run
green against pgstore too, which is what makes it a contract rather than a
description of whatever sqlite happens to do.

| AC | Test |
|----|------|
| 1 | Compile-time capability assertions + `versionServiceFor` returns non-nil on the sqlite build |
| 2 | New `storetest` suite green on both sqlite and pgstore |
| 3 | Multi-face entity: `draft` v1 ≠ `published` v1; matches `states_versioning_test.go` |
| 4 | Rename chain A→B→C with a reused `A`; test fails if the fence is removed |
| 5 | Purge: each guardrail has a negative test |
| 6 | `user_version` 1→2 migration; refusal of a newer db |
| 7 | Existing `go list -deps` CI assertions |
| 8 | End-to-end: history endpoint no longer 501s on sqlite |
| 9 | Sweep-captured rows carry `version-sweep`; sync rows carry the real principal |

**Edge Cases:**
- Entity renamed onto a previously-used id (the fence — verified above)
- Delete + recreate of the same relation triple: fresh lineage, no history merge
- A state-tailed relation edge vs a default-tail one (migration 0012's stitch trap)
- Empty content, empty property map, unicode ids
- Sweep tick racing a purge
- An entity with history that is then deleted (history must survive)

**Negative Tests:**
- Purge a rename row → refused
- Purge while live row holds the content without `--force-live` → refused
- `GetVersion` ordinal < 1 or beyond the lineage → `ErrNotFound`
- Open a `user_version = 99` database → refused with a message naming the cause

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. ~~Recursive CTE may not port~~ — **retired empirically** (see Approach).
2. **Extracting the harness may surface pgstore bugs.** The worlds work
(#1452) rewrote all four pgstore versioning files days ago; a first shared
contract may find divergence. Mitigation: treat any pgstore failure as a real
finding and report it rather than weakening the harness to fit. This is the
accepted cost of the long-term-correct choice.
3. **`AUTOINCREMENT` monotonicity.** SQLite reuses rowids without it; the
lineage fence depends on monotonic `vseq`. Mitigation: `AUTOINCREMENT`
explicitly, with a test that deletes the newest row and asserts the next insert
does not reuse its vseq.
4. **Effort is `l`, likely `xl`.** Seven capabilities over ~2k reference lines
plus a harness extraction. Flagged to the user before starting.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `CLAUDE.md` — the storage-backend section states versioning is postgres-only
and that sqlite "has no versioning yet"; both become wrong
- [x] `docs/sqlite-backend.md` (if present) / the backend comparison table
- [x] `docs/cli-reference.md` — the planning assumption that purge commands are
described generically turned out to be wrong: `rela history`,
`relation-history`, `restore`, `relation-restore`, `history-purge` and
`relation-history-purge` were each marked "PostgreSQL build only", as were
five CLI help messages naming a "PostgreSQL-build feature"

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

Three findings, all addressed in the plan above before implementation
continued. Two were predicted by the Risk Assessment, which is the checklist
working as intended rather than a coincidence.

1. **Significant — the migration ladder moved underneath this plan.** The
approach above targets `user_version` 1→2 with a ladder local to
`sqlitestore`. TKT-S1EVV7 has since split the database file into
`internal/sqlitedb`, which owns opening, PRAGMAs, the single-writer lock AND a
real migration ladder now at v3. The versioning schema therefore becomes rung
**v3→v4 in `sqlitedb`**, not a private v1→v2 step, and AC-6's "migration to 2"
reads as "one ladder rung, tested". Addressed: `migrate.go` as planned is
dropped entirely; the DDL lands in `sqlitedb/versionschema.go` and is shared
between the fresh path and the rung exactly as `projectFilesDDL` already is.

2. **Significant — `ALTER TABLE` cannot live in the shared DDL.** Adding
`rel_record_id` is the one non-idempotent statement in the versioning schema
(no `IF NOT EXISTS` form), and `schemaSQL` re-runs on every open. Worse, that
same `schemaSQL` indexes the column, so on a pre-v4 database it would fail
before the ladder ever ran. Addressed: the column is declared inline in the
`relations` CREATE TABLE for fresh databases, and added by a pragma-probed
`ensureRelRecordIDColumn` on the open path ahead of `schemaSQL` for existing
ones; the rung keeps only the backfill, which is the part that is data rather
than shape. `TestFreshDatabaseHasTheVersioningSchema` pins that the two paths
agree.

3. **Significant — the plan under-scoped the wiring seam (Risk 2's shape,
different file).** "the `!postgres` seam" is one line in the Files list, but
`versionServiceFor` and `stateKVFor` shared a single `//go:build !postgres`
file, so versioning could not be enabled for sqlite without also dragging in
the database-backed state KV that this ticket explicitly puts OUT of scope.
Addressed: the file is split by concern rather than by backend —
`versionsweep_shared.go` (`postgres || sqlite`) holds the backend-neutral
resolvers, `statekv_nodb.go` (`!postgres`) keeps the FSKV for sqlite, so the
two capabilities go opposite ways for stated reasons.

**Predicted risk confirmed (Risk 2).** Extracting the harness did surface a
pgstore defect: `RelationMultiLifetimeRequiresASelector` skipped on pgstore
because the seed helper resolved a lineage id through
`ListRelationLifetimes`, which summarizes VERSION ROWS and so returns the
DEAD lifetime after a delete+recreate. Per the mitigation it was treated as a
real finding and fixed rather than papered over — pgstore gained a
`RelationRecordID` accessor mirroring sqlitestore's, the helper now asks the
live row, and the skip became an assertion. The postgres CI job fails on any
`--- SKIP`, so leaving it would have broken CI.
