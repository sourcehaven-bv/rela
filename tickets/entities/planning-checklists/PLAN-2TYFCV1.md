---
id: PLAN-2TYFCV1
type: planning-checklist
title: 'Planning: Store content hash at write time and add a fingerprint column'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Step 3 (TKT-IQU106C) sums element fingerprints over address ranges in SQL. Every
live row in pgstore and sqlitestore therefore needs its content hash, its
address and its fingerprint stored, and correct, right after every write.

The existing `content_hash` column cannot carry this. Its meaning since
BUG-1DWMYO is "the latest version of this lifecycle has this hash": the sweep
selects rows where it is NULL, triggers clear it on every insert and change, and
a write-time value would stop the sweep from ever capturing creates and updates.
So this ticket adds new columns next to it and leaves `content_hash`, its
triggers and the sweep's capture logic unchanged.

**Scope:**

In scope:
- pgstore migration 0042 and sqlitedb rung v34: per live row `element_hash` (hex canonical hash) and `element_fp0..element_fp3` (the four 32-bit limbs of `reconcile.Fingerprint`, most significant first, bigint / INTEGER), on `entities` and `relations`. sqlite's `marked_*` tables do not get them: a restored row is always rehashed.
- The address is not stored. It is a pure function of the key columns, so it is an index expression: entity `type || '/' || id || CASE WHEN face = '' THEN '' ELSE '@' || face END`, relation `from_id || CASE WHEN from_face = '' THEN '' ELSE '@' || from_face END || '--' || rel_type || '--' || to_id`. One Go constant per backend holds each expression, used by the DDL and by every query, so step 3 spells it identically (needed for index matching). An unhashed row therefore still has its address and can be counted in a range (`count(*)` vs `count(element_hash)`), instead of silently dropping out.
- A covering expression index per table: pg `((expr) COLLATE "C") INCLUDE (element_fp0..3, element_hash)`, sqlite `(expr, element_fp0..3, element_hash)` (BINARY). Plus a partial index on the key `WHERE element_hash IS NULL` for the heal.
- A safety-net trigger per table that clears hash and limbs when a hashed column changes and the statement did not change `element_hash`, so a writer that ignores the columns (raw SQL, or a pg process still on the old binary during a rolling deploy) leaves NULL, never a stale value.
- Every store write path that touches a hashed column fills the element columns in the same transaction: create/update entity, create/update relation, rename (entity ids and incident relations), convert (ReplaceEntityFamily), SwapRelationEndpoints, soft-delete restore (Unmark).
- A heal pass in each backend's version sweep tick that fills rows whose element columns are NULL (the post-migration backlog, and rows an old pg binary changed).

Out of scope:
- The range-sum store capability, its interface and its fs/mem implementation (TKT-IQU106C).
- The redacted view hash (step 4) and `_reconcile` (step 5).
- fsstore and memstore store nothing new: their element is `reconcile.EntityElement(e)` / `reconcile.RelationElement(r)` computed in memory from the live row, which step 1 already provides. Step 3 uses that for their range sums.
- Using `element_hash` to save the sweep a hash computation (possible later optimisation).

**Acceptance Criteria:**
1. No live row has a NULL element hash after a write on pgstore and sqlitestore: storetest checks every stored row after create, update (including a content change the canonical hash folds to the same value), rename, delete, convert, endpoint swap and soft-delete restore.
2. The stored hash equals `canonical.HashEntity`/`HashRelation` of the row as the store reads it back, the database's address expression equals `reconcile.EntityAddress` / `RelationKey.String()`, and the stored limbs equal `reconcile.Element.Fingerprint().Limbs()`.
3. Deleted and renamed-away rows leave no stored element behind (set comparison, not per-row lookup).
4. A raw SQL change of a hashed column that does not set the element columns clears them (trigger), and the sweep heal then fills them. A heal that read a row that changed before its write does not store a stale value (pg xmin guard; sqlite runs the heal inside one write transaction).
5. Database order equals Go byte order: `ORDER BY element_address` returns addresses in `slices.Sort` order for the pairs from `TestAddress_BytewiseOrder`, on pg and sqlite.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R (set reconciliation; recommends the additive SQL
fingerprint).

**Existing Solutions:**
- `internal/reconcile` (step 1): `EntityElement`, `RelationElement`, `Fingerprint.Limbs`. Reused as is.
- pgstore migrations 0020/0021 and sqlitedb v14/v15 (`contentHashDDL`): the trigger-clears, sweep-fills pattern for `content_hash`. The safety-net trigger and the heal reuse it.
- pg sweep's xmin guard (`writeBackEntityHash`) for "row changed since the read": reused for the heal.
- Negentropy keeps per-item fingerprints for incremental range sums (see the research). No library applies; the hash is rela's canonical hash.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Single-row writes set the columns in the write statement itself; bulk writes
rely on the trigger plus a fill.

- Single-row create and update (entity and relation, both backends) compute the element in Go from the row as the store will read it back: properties are `unmarshalProps(marshalProps(p))`, the same decode the read path runs, so the hash equals the hash of a later read. The INSERT or UPDATE sets `element_hash` and the limbs.
- Trigger (pg BEFORE UPDATE OF hashed columns; sqlite AFTER UPDATE OF ..., by rowid): when a hashed column is distinct AND `element_hash` is not distinct from its old value, clear hash and limbs. An old binary or raw SQL leaves `element_hash` untouched, so the row is cleared. A new-binary write whose canonical hash is unchanged (a change the canonical form folds away) is also cleared; the write path then runs the fill for that one key in the same transaction (pg: `RETURNING element_hash IS NULL`; sqlite: an indexed probe after the statement). No insert trigger: an insert either sets the columns or leaves NULL.
- Fill helpers, package functions (not Store methods, for plimsoll), with typed selectors built from package constants (no caller SQL): one row, an entity family, the relations incident to an id, the relations of a type, or "any unhashed, up to N, in key order". A fill reads the selected rows `WHERE element_hash IS NULL` with the store's normal scan, computes, and writes. pg writes in one `UPDATE ... FROM unnest(...)` guarded by `xmin` and `element_hash IS NULL`; sqlite writes per row by rowid inside the transaction.
- Bulk paths call a fill after their statements, in the same transaction: rename (family by new id, relations incident to new id), convert (family), SwapRelationEndpoints (relations of the type), Unmark (family and its restored relations). Unmark strips the element keys from the saved jsonb (pg) or omits the columns from the copy list (sqlite), so a restored row is always rehashed rather than trusting a value saved at mark time.
- sqlite update paths that now run two statements (update, relation update) run inside `s.Tx`, so write and fill commit together.
- Heal: each version sweep tick first fills up to Batch unhashed rows per table (partial index, key order), after pg takes the sweep advisory lock (sqlite: before `versionMu`) and before the projection check. Its errors are logged and never abort the version capture. A full batch with progress sets `more`; a full batch without progress logs a warning, the same rule as `noteBatch`. pg relies on the xmin guard; sqlite runs the heal in `s.Tx`.
- arch-lint: `pgstore` and `sqlitestore` may depend on `reconcile` (the stored element must be the same function the in-memory backends compute).

Alternatives rejected:
- Reuse `content_hash` at write time: breaks the sweep (it selects NULL rows; the 0021 insert trigger clears any inserted hash). Changing that invariant reopens BUG-1DWMYO.
- Always a second statement after each write: doubles non-HOT tuple versions and index maintenance on every write (entities carries about a dozen indexes).
- A stored address column: a rename by an old binary would leave it stale, and a cleared row would vanish from ranges. The expression costs nothing to store and is always right.
- A blocking backfill at open: a large table would delay startup. The heal drains the backlog in batches and also covers old-binary writes. Step 3 compares `count(*)` with `count(element_hash)` per range and fills or refuses a range with unhashed rows.
- A production `store` capability to read stored elements: only tests need it now; step 3 defines the real capability. storetest gets a harness hook instead (`Capabilities.StoredElements`).

Changing `canonical` normalisation or `fingerprintDomain` invalidates every
stored value; the migration comment and storage.md say such a change needs a
migration that clears the columns. `TestFingerprint_Golden` already pins the
encoding.
- A production `store` capability to read stored elements: only tests need it now; step 3 defines the real capability. storetest gets a harness hook instead (`Capabilities.StoredElements`).
- A blocking backfill at open: a large table would delay startup. The sweep heal drains the backlog in batches and also covers old-binary writes. Step 3 must treat a NULL row as "not yet hashed" (it can run the same fill first).

**Files to modify:**
- `internal/store/pgstore/migrations/0042_element_columns.sql` (new)
- `internal/store/pgstore/element.go` (new), `entity.go`, `relation.go`, `convert.go`, `bulkmigrate.go`, `softdelete.go`, `sweep.go`, `export_test.go`, `conformance_test.go`, new `element_test.go`
- `internal/sqlitedb/sqlitedb.go` (columns in schemaSQL, `elementDDL` after the ladder), `migrate.go` (v34 rung), tests
- `internal/store/sqlitestore/element.go` (new), `entity.go`, `relation.go`, `rename.go`, `convert.go`, `bulkmigrate.go`, `softdelete.go`, `sweep.go`, `conformance_test.go`, tests
- `internal/store/storetest/elements.go` (new suite), `storetest.go` (`Capabilities.StoredElements`, RunAll wiring)
- `.claude/rules/storage.md` (one bullet on the element columns)
- `.go-arch-lint.yml` (`reconcile` for pgstore and sqlitestore)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Row content arrives through the existing, already-validated store write paths. No new input surface: the columns are derived in Go from the stored row and never accepted from a caller. SQL uses bind parameters only; table names are literals.

**Security-Sensitive Operations:**
- The stored hash and fingerprint cover the full row, including fields some readers may not see. Served to such a reader they would be an offline oracle for hidden values. This ticket serves them to nobody. The binding rules of `internal/reconcile` (gate before folding; stored hash only for rows where the reader has no hidden field; relations only with both endpoints visible) apply from step 3 on and are restated in the migration and helper docs.
- No change to ACL, visibility or the write authorization path; the columns sit below `entitymanager`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1-3: new `storetest.RunElementTests`, run from RunAll when `Capabilities.StoredElements` is set (pg, sqlite). After each mutation it compares the full set of stored elements with the set computed from `ListEntities(AllFaces)` and `ListRelations`, and asserts no NULL. Mutations: create (implicit and named face), update, canonical no-op change, property types that round-trip differently (date, 2.0, nested map, large int), relation create/update with a tail face, rename with incident relations, delete face, delete family with cascade, relation delete, SwapRelationEndpoints, ReplaceEntityFamily (when FamilyReplace), MarkDeleted + Unmark (when SoftDelete), writes inside `store.Tx`.
- AC4: pg and sqlite backend tests: a raw `UPDATE entities SET content = ...` clears; a sweep tick refills; a raw NULL-out of every row is filled by ticks; pg heal with a raw-SQL content change between read and write (test hook) keeps NULL, so the xmin guard is what rejects it; a raw jsonb-equal but byte-different properties write (`1` to `1.0`) keeps an element that still equals a fresh read; the fill does not clear `content_hash` and the sweep's `content_hash` write-back does not clear the element; an unhashable row does not block version capture.
- AC5: backend test ordering the address expression from the database against `slices.Sort` and against `reconcile.EntityAddress` / `RelationKey.String()`, with non-ASCII ids and faces; pg asserts `server_encoding = 'UTF8'`.
- Migration: sqlite opens a database stamped v33 and migrates to v34; pg migration applies through the per-schema harness.

**Edge Cases:**
- Faced entity and faced relation tail: address carries `@face`.
- Rename of an id with many incident relations: one batched pg UPDATE.
- Unmark of a family marked before the migration: restored rows have NULL columns, filled in the same transaction.
- A content change equal under canonical normalisation: cleared and refilled, never left NULL.
- Limb values up to 2^32-1 in bigint/INTEGER: non-negative, sum-safe (bound documented in step 1).

**Negative Tests:**
- A raw old-binary-style UPDATE without element columns must not leave a stale element (cleared to NULL).
- A heal racing a writer must not store a stale element.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- A write path missed: the trigger turns it into NULL (detectable) rather than stale; storetest compares the whole stored set after every mutation kind; the heal repairs it.
- Write latency: one SELECT and one UPDATE more per write, inside the existing transaction. Accepted.
- Migration cost: pg ADD COLUMN without default is catalog-only; each index build scans the table once. sqlite ADD COLUMN is cheap; the index build scans once.
- Hash mismatch between write time and read: the input is hashed after the same JSON decode the read path runs; storetest compares every stored hash with a fresh read, including round-trip-sensitive values.
- Index builds in 0042 block writes for one table scan each; noted in the migration.

Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed (storage internals; `.claude/rules/storage.md` gets the developer note, the migration files carry the rationale)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-NELOFS0, RR-1YO1UJ4, RR-YPSGGL1, RR-BDETJJG
(significant, addressed in the plan); RR-4WX8ORJ, RR-8VZOCTF, RR-22I12RC,
RR-5D6517H (minor); RR-FQX8KBJ (nits).
