---
id: PLAN-2TN7LF
type: planning-checklist
title: 'Planning: Replace the shape-hash migration chain with a committed applied-list; enable data-only migrations'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Revised 2026-09-19 after design review.** Five findings; all resolved. The
> significant change: **embedded projections STAY in migration files** — only the
> `from:`/`to:` hashes are removed (RR-SJISFW). See Design Review.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:

- A **migration-state service** with per-backend storage implementations,
replacing today's `state.KV` marker (`migration/state.json`). Jeroen,
2026-09-19: "all should get a migration service that has different storage
backends - no more kv."
- fs backend persists to a **committed** `migrations/applied.json`.
- postgres and sqlite persist to their own table, keeping **per-store
independence** (a documented guarantee — see Risks).
- Migration files lose **`from:` and `to:` only**. `from_projection:` and
`to_projection:` are RETAINED (RR-SJISFW).
- Naming moves from `%04d-name.yaml` to `YYYYMMDDHHMMSS-name.yaml`.
- Data-only migrations become expressible — the point of the ticket.
- Conservative bootstrap **on the CLI**; explicit baseline command as the
auditable override.
- **Only CLI commands persist migration state; the server is read-only**
(Jeroen, 2026-09-19).
- Dual-write of the legacy `state.KV` marker for one release (RR-2HRIHC).
- Conformance harness (`migstatetest.RunAll`) so all backends meet one contract.

OUT:

- **Removing the embedded projections** — struck after design review. They are
what `validateDeltasResolved` and the 13 step `Validate(from, to)` impls run on;
removing them reopens BUG-TMGWIN and breaks historical step validation. Not
required for data-only migrations.
- Making needs-migration **fatal** at server boot — separable behaviour change,
belongs with TKT-5RAW5I (RR-T6EJGG).
- Drift-tier audit record — TKT-5RAW5I.
- Golden-hash pin / projection format version — TKT-ZV8CH0 (`depends-on`).
- The additive/drift/needs-migration classifier itself — untouched.
- `internal/migration` (config-file *syntax* migrations) — unrelated.

**Acceptance Criteria:**

1. A migration file with no shape delta between its projections applies, is
recorded, and is skipped on re-run. (Today: rejected at parse by `from == to`.)
2. `migstatetest.RunAll` passes for file, pg, sqlite and mem backends.
3. Two postgres tenants at different positions migrate independently.
4. A server process never writes migration state; a CLI command does.
5. Non-empty `migrations/` with no recorded state refuses **on the CLI**; the
server serves.
6. A sqlite project keeps its applied-list in `rela.db`, not stranded on disk.
7. `validateDeltasResolved` still refuses a face-adoption file with no
`migrate_face` step — i.e. `TestParseFile_RefusesUnmigratedFaceAdoption` passes
unmodified.
8. A rollback to the previous binary finds a current legacy marker.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-273DPJ (existing). Its bookkeeping section already
recommended this: "Applied ledger … migration name + content hash + applied-at +
outcome." Name-keyed, not shape-edge-keyed. The shape-hash chain arrived during
implementation, not from that research. External survey (Rails, Django, Alembic,
Flyway, Liquibase, Atlas, Prisma, EF Core, Avro, buf, GraphQL Inspector) done
in-session; recorded on TKT-XCJ0Y2.

**Existing Solutions:**

- **The model**: every mainstream tool tracks applied migrations by NAME (Rails
`schema_migrations`, Django `django_migrations`, Flyway, Liquibase, goose,
golang-migrate). None uses a schema content-hash as position. Data-only
migrations are routine in all of them *because* of this.
- **Committing the pointer on fs**: Rails `db/schema.rb`, EF Core
`ModelSnapshot.cs` — both accept the generated-file-in-git merge cost.
- **Timestamps**: Rails, EF Core. Avoids the concurrent-PR collision `%04d` has,
already live here as BUG-TY2XQC.
- **In-codebase pattern — `internal/comments` (TKT-OGTVJW)**, the mandated shape
for backend-selected storage:
  - `comments.Store` + `filecomments`/`pgcomments`/`sqlitecomments`/`memcomments`.
  - Backend chosen by the RECIPE via `backendOverrides.commentStore` →
`buildComments` (`internal/appbuild/comments.go:45`, `appbuild.go:1783`); nil
selects file.
  - Each DB backend declares its OWN narrow `DBTX`
(`internal/comments/pgcomments/pg.go:56-81`); arch-lint forbids importing
`internal/store`.
  - `filecomments.New(base storage.FS, root string)`
(`internal/comments/filecomments/file.go:65`) — `SafeFS` **then** `RootedFS`.
  - `commentstest.RunAll` (`internal/comments/commentstest/commentstest.go:47`)
is the conformance-harness template.
- **`state.KV` is the wrong home.** Its neighbours are node-local cache state.
Migration state describes the DATA, so it belongs with the data — the same "is
this about the machine or about the content?" test CLAUDE.md records for
splitting comments and versioning out of `state.KV`.
- **`config.Loader` is read-only** (`internal/config/config.go:23-49`). The
applied-list cannot be written back through the seam `LoadDir` reads it from.
`rela migrate gen` already writes directly via `storage.FS` + `Paths.Root`
(`internal/cli/migrate_data.go:158-167`) — the precedent to follow.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

A `datamigration.StateStore` seam modelled on `comments.Store`:

```go
// Nil: rejected — constructors validate required collaborators.
type StateStore interface {
    Load(ctx context.Context) (*State, error)  // (nil, nil) when absent
    Save(ctx context.Context, s *State) error
}

type State struct {
    FormatVersion int              // RR-2HRIHC: absent vs newer-than-me
    Applied       []AppliedEntry   // MigrationName + applied-at
    Projection    json.RawMessage  // the shape the data conforms to
}
```

Implementations, one package each, none importing `internal/store`:

- `filemigstate` — `migrations/applied.json` at the project root, committed.
`New(base storage.FS, root string)`; SafeFS then RootedFS.
- `pgmigstate` — own `DBTX`, table in the tenant's schema (new
`pgstore/migrations/0016_*.sql`).
- `sqlitemigstate` — own handle, table in `rela.db`. **Required, not optional**:
`configloader_sqlite.go:15-19` puts project config in the DB so a shipped
`rela.db` is self-contained; a disk file would strand the applied-list.
- `memmigstate` — tests / memory backend.

Wiring: `backendOverrides.migStateStore`, set by the postgres and sqlite
recipes; nil selects file. Mirrors `commentStore` exactly.

**File format.** `from:`/`to:` are removed; `hashRe` and the three hash refusals
(`file.go:69-77`) go with them, which is what unblocks data-only migrations.
`from_projection:`/`to_projection:` STAY, so `file.go:112`
(`s.Validate(fromProj, toProj)`) and `file.go:136-213` (`validateDeltasResolved`
/ `resolvingSteps`) are **unchanged** — no signature churn across `steps.go`,
and the BUG-TMGWIN guard keeps its input. `file.go:89-94`'s hash↔projection
integrity check goes (nothing left to check against).

**Read/write split (Jeroen, 2026-09-19).** The gate still *evaluates* at every
process start on every backend — needs-migration still logs, drift still logs —
but **persistence is CLI-only**:

- No server writes a git-tracked file, so no surprise working-tree diff and no
writable-project-dir requirement on a deploy box.
- The multi-instance adoption race disappears rather than being mitigated.
- A server may serve with an unrecorded *additive* change: harmless by
definition, recorded on the next CLI invocation.
- `gate.go:54-57`'s "run Evaluate only from write-capable processes" note
inverts — update it.
- Per RR-T6EJGG: the verdict is **surfaced through the server status/health
surface**, not only logged; needs-migration is **not** fatal in this ticket; the
conservative bootstrap refusal is **CLI-only** so clone-and-deploy cannot
deadlock.

**Legacy transition (RR-2HRIHC).** The new writer also writes the legacy
`state.KV` marker for one release (best-effort, never fatal); the new reader
prefers the new location. A rollback then finds a current marker rather than an
absent one, which `gate.go:109-114` would otherwise silently re-baseline.
`FormatVersion` lets a future reader distinguish absent from newer-than-me.

Ordering/position: the applied list alone decides what runs. `LoadDir` already
sorts by filename (`file.go:309`) and timestamps sort chronologically.
`Resolve`'s edge-walk collapses to a filter; the free-edge machinery goes, and
multi-tenant catch-up falls out because each store has its own list.

**Alternatives considered:**

- *One committed file for all backends* — breaks per-store independence.
- *Committed manifest + `state.KV` position* — two files to keep coherent, and
keeps migration state in `state.KV`, explicitly ruled out.
- *Extend `config.Loader` with a write capability* — larger blast radius; `gen`'s
direct-FS write is the established precedent.
- *Remove embedded projections* — rejected by design review (RR-SJISFW).
- *Split into two tickets* — considered (RR-GPC7NH), declined; the riskiest item
left scope so the remainder is smaller than when the split was proposed.

**Files to modify** (from the inventory):

- `internal/datamigration/`: `marker.go` (→ state service; `ledgerKey` STAYS,
only `markerKey` moves), `resolve.go` (edge-walk → applied filter; `:10-33` doc
obsolete), `file.go` (`:36-39` drop From/To fields, `:48-55` drop the two YAML
keys, `:57` `hashRe` deleted, `:69-77` three refusals deleted, `:89-94`
integrity check deleted — **`:112` and `:136-213` UNCHANGED**), `generate.go`
(`:46-47` stop emitting hashes; `:62-73` and `:349-367` RETAINED; `:75` naming;
`:381` `nextIndex` deleted), `gate.go` (`:109-114`, `:219-234` `Describe()`
strings, `:54-57` doc), `run.go` (`:98-103`, `:138`, `:174`
`NewMarker(f.ToProjection, …)` — still available since projections stay,
`:192-193` audit summary), `doc.go` (`:9-12`, `:24-27`).
- New: `MigrationName` validated type (RR-J064T6);
`internal/datamigration/migstatetest/`; the four backend packages.
- `internal/appbuild/datamigration.go:34-42` (+`Root`/FS params),
`appbuild.go:1599`, `appbuild_postgres.go`, `configloader_sqlite.go`.
- `internal/cli/migrate_data.go` — `:267` hard break; strings at `:107`, `:109`,
`:135`, `:151`, `:191`.
- `internal/store/pgstore/migrations/0016_migration_state.sql`; sqlite ladder.
- `internal/datamigration/lock.go:52-56` — comment half-wrong after the move.
- Docs: `docs-project/entities/guides/GUIDE-data-migration.md` (`:32`, `:66-83`,
`:87-88`, `:221`, `:247-251`, `~:272-285`), `GUIDE-cli-reference.md` (`:1641`,
`:1653`) — **edit the entities, not the generated .md** — plus the
postgres-backend per-tenant paragraph and CLAUDE.md's data-migration bullet.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `migrations/*.yaml` — operator-authored, in-repo; trust boundary is the
operator shell, same as `db migrate`. Parsing stays strict
(`KnownFields(true)`); step targets keep validating against the file's own
embedded projections.
- **Migration names (RR-J064T6).** Now semi-trusted: they round-trip through a
hand-editable committed file and are matched against directory entries. A
validated `MigrationName` type (unexported field, verifying constructor, per the
`param-contract` convention) accepts **only**:

  ```text
  ^[0-9]{14}-[a-z0-9]+(-[a-z0-9]+)*\.ya?ml$
  ```

Reject, never sanitise. Enforced on **both** sides — `LoadDir`'s listing
(replacing the suffix-only filter at `file.go:300-308`) and applied-list parsing
— so both populations share one alphabet and the comparison means something.
Lowercase-ASCII-only forecloses macOS case-insensitivity collisions and Unicode
NFC/NFD pairs. Length capped at 128 bytes.
- `migrations/applied.json` — reject unknown fields. A corrupt file must
**refuse clearly** rather than be treated as absent: today's
treat-corruption-as-absent (`marker.go:51-69`) was safe when re-bootstrapping
was silent, and is not once bootstrap is conservative.
- File paths — `RootedFS` containment on the file backend.

**Security-Sensitive Operations:**

- Raw-store writes bypassing validation/automations/ACL — unchanged; operator
shell trust, still audited.
- The migration lock (TKT-CPCBR7) still serializes CLI runners and the GC sweep.
With the server no longer persisting, the gate's contended-adoption skip loses
its server-side reason to exist.
- Writes stay atomic via `SafeFS`.
- Config is not a secret (CLAUDE.md): migration names and step contents are
operator-authored config. Audit records keep naming counts, never content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** (numbering matches Acceptance Criteria)

1. Data-only file applies → recorded → skipped on re-run.
2. `migstatetest.RunAll` across all four backends (pg DB-gated on
`RELA_TEST_DATABASE_URL`, like `storetest`).
3. Two postgres schemas at different positions migrating independently.
4. Server assembly performs no write — assert state untouched after boot with a
pending additive change.
5. CLI refuses on non-empty `migrations/` + absent state; baseline adopts; the
server does NOT refuse.
6. sqlite: applied-list round-trips through `rela.db`; nothing written to disk.
7. `TestParseFile_RefusesUnmigratedFaceAdoption` and
`TestResolvingSteps_CoversEveryMigrationDeltaKind` pass **unmodified** — the
regression guard for RR-SJISFW.
8. Legacy-marker dual-write: after a new-binary apply, the legacy `state.KV`
marker reflects the same position.

**Rewrites required** — materially smaller than pre-review, since `mustFileYAML`
keeps emitting both projections and only sheds the two hash lines. Still to fix:
`run_test.go:255-319` `TestResolve_ChainAndFreeEdges` (7 subtests, obsolete);
`generate_test.go:32,124-133` (`%04d` names); `lock_test.go:368,383` (direct
`markerKey` access); `file_test.go:20,23,48-64` (hash assertions;
`TestParseFile_MissingProjection` **survives**); `gate_test.go:18,29,62,107`.
`migrateface_test.go` is now **untouched**.

**No CLI tests exist** for `migrate status|gen|data|gc`. Add coverage here.

**Edge Cases:** empty `migrations/`; applied-list naming a missing file (Rails
prints `NO FILE` — pick an analogue); duplicate timestamps (distinct slugs sort
deterministically); clock skew; hand-edited list; a file present but unlisted
and older than listed ones (the merge-order hazard timestamps introduce); no
project-root equivalent of `nopKV` (`appbuild.go:2238-2252`).

**Negative Tests:** corrupt `applied.json`; unknown JSON/YAML fields; a name
failing the allowlist (each rejected class: traversal, uppercase, overlong,
non-timestamp prefix); one unparseable file among several (`file.go:317-319`
aborts the whole load today — decide and pin either way).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) — **l**

**Risks:**

- **Per-store independence is a documented guarantee.**
`docs/postgres-backend.md:469-470`. Mitigated by construction (per-backend
service); any future move toward one shared file must re-read it.
- ~~`validateDeltasResolved` loses its input~~ — **eliminated** by retaining the
embedded projections (RR-SJISFW). Pinned by criterion 7.
- **Rollback across the `state.KV` transition** — mitigated by dual-write for one
release plus `FormatVersion` (RR-2HRIHC).
- **Losing the shape-edge walk removes a backstop.** `resolve.go:53` skips a file
whose `to` is already reached even when the applied list is wrong. Afterwards
step idempotency is the only guard — state it plainly in the docs. Note
idempotency is not uniform: a `lua:` step is idempotent only if the operator
wrote it so.
- **A needs-migration store serves indefinitely** with no fatal signal
(RR-T6EJGG) — mitigated by surfacing the verdict; making it fatal is deferred.
- **`applied.json` merge conflicts** are the intended coordination mechanism
(Atlas's `atlas.sum` argument) but need a documented resolution recipe.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A at
      planning time: the checklist is auto-created on the transition to
      `in-progress`; documentation impact is enumerated below)

**Documentation Impact:**

- [x] `GUIDE-data-migration.md` — file format (hashes gone, projections stay),
workflow, "Two hashes on purpose" rewritten, marker location at `:32`, the
idempotency-is-now-the-only-guard note, the merge-conflict recipe.
- [x] `GUIDE-cli-reference.md` — `:1641`, `:1653`; the new baseline command.
- [x] postgres-backend guide — the per-tenant data-migration paragraph.
- [x] CLAUDE.md — the data-migration bullet.
- [x] Project-layout listing — `migrations/applied.json` is a new committed file.
- Generated `docs/*.md` are rebuilt from the entities, never edited directly.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Status |
|---|---|---|
| RR-SJISFW | critical | addressed — projections retained; only hashes removed |
| RR-T6EJGG | significant | addressed — surface the verdict; not fatal; CLI-only refusal |
| RR-J064T6 | significant | addressed — `MigrationName` allowlist, both sides |
| RR-2HRIHC | significant | addressed — dual-write one release + `FormatVersion` |
| RR-GPC7NH | minor | wont-fix — one ticket (Jeroen); risk left scope with RR-SJISFW |
