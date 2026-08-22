---
id: PLAN-OX2A9U
type: planning-checklist
title: 'Planning: Data migration system: shape hash, compatibility classifier, declarative migrations, GC sweep'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope (v1):
- `metamodel.ShapeProjection` + `Hash()` — the semantic data-shape fingerprint (entities/properties incl. defaults, custom types, relation types incl. from/to/cardinality/properties). id prefixes EXCLUDED (A5/RR-JPYXQ9). `RenderProjection` untouched.
- `metamodel.CompareShapes(from, to) → Report` — three-tier classifier (additive / drift / needs-migration), incl. possible-rename pair detection; default-only changes tiered additive (A7).
- New package `internal/datamigration`: migration file format (from/to shape hash + embedded FROM projection + steps), parser, chain resolver (migrations = mandatory edges, compatibility = free edges), step executors, generator (shape diff → draft YAML), GC engine + drift ledger.
- Declarative steps v1: `rename_property`, `rename_entity_type` (dedicated step, A3), `rename_relation_type`, `map_values`, `set_default` (only_missing), `convert` (built-in coercions), `drop_property`, `drop_entities`, `drop_relations`, `lua` (pure transform).
- Synchronous version capture for destructive/rename steps and GC deletions via optional VersionWriter/RelationVersionWriter capabilities (A1).
- fsstore fix: `updateEntity` relocates the file on type change; storetest conformance case pins type-change-on-update across backends (A3).
- Applied-state marker `migration/state.json` + drift ledger `migration/drift.json` (keyed by schema name, A6) in `state.KV`.
- CLI: `rela migrate status|gen|data|gc` subcommands (bare `rela migrate` unchanged, still service-free).
- Gate: hash compare → adopt / notice / warn, at startup AND on metamodel hot-reload (A4), verdict via atomic.Pointer; wired in appbuild for server + write-capable CLI.
- GC sweep goroutine with server lifecycle (version-sweep pattern) + manual `gc --apply` + scheduler-invokable engine; audit sink as nil-rejected constructor dep (A8).
- Works on fsstore, memstore, pgstore; pg parts (advisory locks) tagged like existing recipes.
- Docs: new `docs/data-migration.md`, updates to cli-reference, postgres-backend, CLAUDE.md pointer.

NOT in scope (v1):
- `split_property`/`merge_property` declarative ops (Lua covers).
- Automatic backfill of new required properties (gen emits commented `set_default`).
- Strict/blocking gate as default (config option only).
- GC of stale enum values inside data (`map_values` territory).
- Rewriting historical `entity_versions` rows (history stays as-captured; that's the feature).
- UI for migrations in the data-entry SPA (CLI + startup banner/log only).
- id-prefix migration (`reprefix_ids`) — prefixes excluded from the shape hash in v1 (A5).

**Acceptance Criteria:**

1. Cosmetic schema edits (views, descriptions, ordering changes to non-shape config, automations, colors, id prefixes; default-only edits are additive) do not demand migration. *Test: table-driven — edit each cosmetic field on a fixture metamodel, assert hash unchanged (or additive-tiered); edit each shape field, assert hash changes.*
2. Additive change auto-adopts silently: new entity type / optional property / enum value → gate updates marker, no warnings. *Test: gate unit test with marker at old projection.*
3. Drift change adopts with notices: deleted property survives in data; delete+add same-type pair yields the possible-rename warning naming both properties. *Test: classifier unit tests + gate integration test asserting notice text.*
4. Needs-migration change without a migration: gate does NOT adopt, warns with concrete deltas; writes still succeed with soft warnings (current behavior preserved). *Test: gate test + entitymanager write test unchanged.*
5. `rela migrate gen` emits rename/map_values/convert guesses with GUESS/TODO annotations; deletions only as comments; FROM projection embedded, hash-consistent (parse rejects mismatch). *Test: golden-file tests over fixture schema pairs + integrity negative test.*
6. `rela migrate data`: dry-run prints per-step affected counts + validation delta; `--apply` transforms all entities; re-running after a simulated mid-run crash completes correctly (idempotent steps); one audit record; marker updated only on full success. *Test: integration test on memstore/fsstore; crash simulated by aborting after N writes and re-running.*
7. Chain resolution bridges compatible gaps (store at H1, adopted-only H1→H2, migration H2→H3 applies). *Test: resolver unit test with synthetic projections.*
8. Lua step is a pure transform applied by the engine; transforming an entity whose type is unknown to the NEW schema still applies (no entitymanager validation). *Test: integration test with a lua step and an unknown_type entity.*
9. GC sweep: respects grace period; skips tick while needs-migration deltas pending (latest reload-aware verdict); drops ledger entries when schema re-adds a property; audits deletions; captures pre-delete versions on pg (A1); works on fs and pg. *Test: engine unit tests with injected clock; pg tick test DB-gated.*
10. Multi-tenant pg: two schemas at different hashes migrate independently; advisory locks schema-scoped. *Test: DB-gated test with two schemas on one database.*
11. `rename_entity_type` leaves no orphan file on fsstore (storetest-pinned type-change-on-update); destructive steps produce version snapshots on pg. *Test: storetest conformance case + DB-gated capture test.*
12. Metamodel hot-reload re-runs the gate; a mid-run incompatible edit surfaces notices/warnings without restart. *Test: gate test driving the reload subscriber.*
13. All three backends pass; postgres suite gated on `RELA_TEST_DATABASE_URL` as usual.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (skipped: design was developed interactively with the operator in-session; the codebase survey, options, and tradeoffs are captured in the ticket body TKT-0C57FS, which serves as the design record)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — design record lives in TKT-0C57FS (full survey + design
in ticket body).

**Existing Solutions:**

- Prior art in-repo (reused, not reinvented):
  - `metamodel.RenderProjection` + length-prefixed hasher (`internal/metamodel/projection.go:113`) — the hashing discipline ShapeProjection copies; RES-4ILUJZ established content-addressed schema snapshots for versioning.
  - `internal/migration` (config YAML rewrites) — deliberately NOT extended; interface (`func(*yaml.Node) error`) doesn't fit data migration. Stays as step 0.
  - `pgstore.Migrate` (embedded SQL, advisory lock) — the DDL layer; its lock pattern (`pg_advisory_xact_lock(key, hashtext(current_schema()))`) is reused for run serialization.
  - Bulk rewrite pattern: `internal/cli/normalize.go:31-55` (collect-then-write, dry-run); `internal/renametype` as reference only (NOT delegated — A3).
  - Store-level op + explicit audit: `internal/cli/history_purge.go` + `writeServices.Audit` (`cli_wiring.go:47`).
  - Sweep lifecycle: pgstore version sweep (`sweep.go`) — goroutine start/stop, advisory lock, debounce, attribution fallback conventions.
  - Version capture capabilities: `VersionWriter`/`RelationVersionWriter` type-asserts + entitymanager `version_hook.go` (the pattern A1 mirrors).
  - Pre-flight scanner: `schema.ValidateEntityProperties` (`internal/schema/validate_properties.go:24`).
- External reference implementations: Django migrations (autogenerated best-guess ops + reviewable files + applied-state table), Rails ActiveRecord (versioned chain), Kubernetes CRD storage versions (declared conversion + lazy rewrite). rela's twist: version = semantic content hash (not a counter), compatibility tiers make most changes migration-free, and GC-with-grace replaces eager destructive rewrites.
- Libraries: none needed — hashing (stdlib sha256), YAML (yaml.v3 already vendored), Lua (existing `internal/lua` runtime + `lua.Cache`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Full design in TKT-0C57FS body (8 sections + design-review amendments A1–A8).
Summary of the load-bearing choices:

1. **Two hashes coexist**: `ShapeProjection` (migration identity) is a sibling of `RenderProjection` (version rendering identity), NOT a modification — RenderProjection's stability is load-bearing for `schema_versions` dedup.
2. **Classifier drives everything**: additive → silent adopt; drift → adopt + notice (incl. GC deadline and possible-rename pairs); needs-migration → warn, don't adopt. Warn-by-default keeps DEC-HWZHA's soft posture; `strict` config upgrades to refuse-writes.
3. **Migrations are hash-to-hash edges; compatibility gaps are free edges** in chain resolution — migration files embed the FROM projection (A2) so the resolver and plan-time validation are self-contained.
4. **Every step idempotent by construction**; runs non-atomic; recovery = re-run; marker written only after full success.
5. **Lua step = pure transform** (entity table in, patch out, engine applies) — no `lua.Mutator`, so no entitymanager validation/automations mid-migration.
6. **Execution = raw batched store writes** under `system:migration` principal + `store.WithAttribution`, one explicit audit record, pg advisory lock mutually exclusive with the version sweep; destructive/rename steps capture versions synchronously via optional capabilities (A1).
7. **Gate runs at startup AND on metamodel hot-reload** (A4), verdict via atomic.Pointer.
8. **GC sweep with schema-name-keyed drift ledger + grace period** (default ~30d, ON by default); never runs while needs-migration deltas are pending; audit sink is a constructor dep (A8).

Alternatives rejected:
- Extending `RenderProjection` with relations → churns `schema_versions` semantics and couples two unrelated stability contracts.
- Numbered migration sequence (Rails-style) → doesn't survive multi-tenant stores at different points, and forces no-op migrations for compatible changes.
- Lua with full WriteDeps/Mutator → writes would traverse entitymanager (validation rejects unknown_type, automations fire) — wrong mid-migration.
- Migration via entitymanager per-entity → same problem + ACL/automation side effects; sanctioned exception is operator-shell raw store access (db migrate / history-purge precedent).
- Immediate destructive cleanup on schema delete → replaced by grace-period GC after operator feedback; migrations CAN delete explicitly (operator control).
- One giant Tx per run on pg → forbidden (stalls all writers; CLAUDE.md Tx rule). Batched small Tx groups instead.
- Delegating `rename_entity_type` to `internal/renametype` → it rewrites schema.yaml, which has already changed by migration time (A3).
- Projection sidecar files → embedded FROM projection is self-contained (A2).

**Files to modify:**

New:
- `internal/metamodel/shapeprojection.go` (+ test) — ShapeProjection, Hash, CompareShapes, Report.
- `internal/datamigration/` — `file.go` (format+parse+projection integrity), `resolve.go` (chain walk), `steps.go` (executors), `luastep.go` (pure-transform runner via `script.Engine.ExecuteCode`), `generate.go` (differ → draft YAML), `run.go` (runner: dry-run/apply, batching, audit, version capture), `gc.go` (engine + drift ledger), `state.go` (marker read/write), `gate.go` (verdict, reload subscription), tests throughout.
- `internal/cli/migrate_data.go`, `migrate_gen.go`, `migrate_gc.go`, `migrate_status.go`.
- `docs/data-migration.md`.

Modified:
- `internal/cli/kong.go` — `requiresProject` full-command-path matching (`migrate data|gen|gc|status` get services; bare `migrate` stays service-free); register subcommands under `MigrateCmd`.
- `internal/store/fsstore/entity.go` — relocate file on type change in `updateEntity`; `internal/store/storetest` — type-change-on-update conformance case (A3).
- `internal/appbuild/` — gate wiring (startup + reload subscription); GC sweep lifecycle wiring (fs + pg recipes; audit sink injection per A8; version-capture capability pass-through per A1).
- `internal/store/pgstore` — advisory lock key export or small capability for migration/GC lock (no schema change; no new tables).
- `internal/scheduler` or scheduler docs — GC engine invokable as scheduled task (may be docs-only if engine is exported).
- `docs/cli-reference.md`, `docs/postgres-backend.md`, `CLAUDE.md` (pointer + rules), `.testcoverage.yml` (floor for new package), arch-lint rules (datamigration may import store/metamodel/state/script; nothing may import it except cli/appbuild).

Dependencies: `internal/metamodel`, `internal/store` (+ `storetest`),
`internal/state`, `internal/schema` (pre-flight),
`internal/script`+`internal/lua` (pure transform), `internal/audit`,
`internal/principal`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `migrations/*.yaml` — operator-authored, same trust class as `schema.yaml`/`scripts/` (config is not secret; contents are operator-controlled). Validation: strict YAML schema (unknown step types/fields rejected at parse, allowlisted step vocabulary), `from`/`to` must be well-formed hex hashes, embedded FROM projection integrity-checked against the `from` hash, step targets (entity types, properties) validated against the embedded FROM projection at plan time — a step naming an unknown type/property is a hard parse error, not a runtime skip.
- Lua transform scripts — operator-authored, run in the existing sandboxed `internal/lua` runtime with read-only env; script paths resolved under the project root only (reuse `storage.RootedFS` / `isSafePathSegment` discipline — no absolute paths, no traversal).
- Entity content — attacker-influenceable in principle; transforms treat values as data (no eval of values, coercions are typed parsers). Lua sees values as plain Lua values.
- `state.KV` marker/ledger — keys are fixed constants (pass `state.ValidateKey` by construction); JSON parsed defensively, corrupt marker → treated as absent with a warning (re-bootstrap), never a crash.

**Security-Sensitive Operations:**

- Trust boundary is the operator shell (like `db migrate`, `history-purge`): no ACL evaluation on the migration read/write path — documented explicitly; NOT reachable from server API/MCP surfaces (CLI + gate only; gate is read-only except the marker write).
- Raw store writes bypass visibility wrappers by design (a redacted read-modify-write would clobber hidden fields — same rationale as entitymanager's raw write-prep reads).
- Audit records carry names/counts/hashes, never entity content (history-purge convention).
- ACL ceiling guard untouched: no new role resolution paths; `internal/datamigration` does not import `internal/acl`.
- Attribution: real `system:migration` / `system:gc-sweep` principals set at the boundary; never a zero/unknown principal translated (RR-U964M0 rule).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** mapped inline to acceptance criteria above (AC1–AC13).
Integration approach: end-to-end tests in `internal/datamigration` against
memstore + fsstore fixtures (real project dir with schema.yaml v1 → v2, entities
on disk); pg paths (advisory locks, two-schema independence, sweep tick, version
capture) DB-gated on `RELA_TEST_DATABASE_URL` via `just test-postgres`.
CLI-level tests for gen golden files and dry-run output.

**Edge Cases:**

- Empty project / zero entities: migration applies trivially, marker still updated.
- No marker (fresh or pre-feature project): bootstrap adopts current hash; never blocks.
- Corrupt marker/ledger JSON: warn + re-bootstrap (marker) / rebuild from classifier report (ledger); never crash startup.
- Migration file whose `from` matches nothing reachable: clear error naming current hash and expected hash.
- Embedded projection whose hash ≠ `from`: parse error (tamper/corruption guard).
- Two migration files with same `from` (fork): error, require linear resolution (v1: refuse ambiguity).
- Step targeting a property absent from every entity: 0-count no-op, reported in dry-run.
- `map_values` value not in mapping: left untouched (idempotent), counted as "unmapped" in report.
- `convert` unparseable value: policy = leave + report (never drop data); listed in dry-run and apply summary. `convert` on `list: true` properties applies per element.
- Unicode/odd property names and entity ids: pass through hasher (length-prefixed, safe) and YAML round-trip.
- Crash mid-apply: re-run completes (AC6); marker not written → gate still reports pending.
- Concurrent `migrate data` on pg: second blocks/fails on advisory lock; on fs: flock or documented single-operator assumption (decide in implementation; test whichever).
- Sweep tick racing migration: lock mutual exclusion (pg test).
- Schema edited mid-run under a live server: gate re-evaluates on reload (AC12); sweep uses latest verdict.
- Clock skew / grace period: injected clock in GC tests; first-seen never in the future.
- Huge graph: batching bounds memory (collect ids, stream batches); no whole-graph Tx.

**Negative Tests:**

- Unknown step type / unknown field in step → parse error naming file+step.
- Bad hash format in from/to → parse error.
- Lua transform returning non-patch (wrong type, unknown property for target type under NEW schema) → step error naming entity id, run aborts before marker write.
- Lua transform attempting I/O or writes → sandbox denies (existing lua runtime guarantees; pin with one test).
- `gc --apply` while needs-migration pending → refuses with explanation.
- `drop_entities` on a type still present in the new schema → parse-time error (only types unknown to the new schema are droppable in v1).
- `migrate data --apply` when store hash already at target → clean no-op exit 0.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Wrong migration destroys data** → dry-run default with per-step counts + validation delta; idempotent steps; recoverability via entity_versions (pg, incl. synchronous capture for deletes/renames per A1) / git (fs); audit trail.
2. **Rename-as-delete+add slips through auto-adoption** (inherent diff ambiguity) → loud drift notice with GC deadline; grace period before any deletion; gen pair-detection. Accepted residual risk, documented.
3. **GC deletes data an operator still wanted** → grace period (default ~30d), ledger removes entries when schema re-adds, sweep skips while migrations pending, audited, recoverable from versions/git; migrations can pre-empt with explicit drops.
4. **fs has no cross-process lock** (state.KV no CAS) → pg uses advisory locks; fs documents single-writer operator assumption (consistent with fsstore's existing single-writer nature); consider lock file in implementation.
5. **Version-sweep interaction** — bulk rewrite creates N version rows → intended (real content change); content-hash dedup makes no-ops free; lock exclusion prevents mid-run capture; attribution correct via WithAttribution; deletes/renames captured synchronously (A1).
6. **Event flood on bulk write** (search reindex, SSE) → batching; subscriber channels already lossy-on-full with catch-up (pg watcher re-snapshot).
7. **Scope creep** — 4 subsystems (projection, classifier, migration engine, GC) → implementable in reviewable stages behind one ticket: (a) ShapeProjection+classifier+marker+gate (incl. reload hook), (b) migration files+runner+gen (incl. fsstore type-change fix + version capture), (c) GC sweep. Each stage lands green.
8. **`requiresProject` change regresses bare `rela migrate`** (must keep working on broken projects) → explicit regression test for service-free bare migrate.
9. **fsstore type-change fix alters store contract** → storetest conformance case pins the new behavior across all backends before the step uses it.

Effort: **l** (matches ticket property). Stage (a) ~s, (b) ~m, (c) ~s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] New `docs/data-migration.md` — concepts (shape hash, tiers, grace period), file format, step reference, workflows (gen → review → apply), GC policy.
- [x] docs/metamodel.md — pointer: schema evolution section.
- [x] docs/cli-reference.md — `rela migrate status|gen|data|gc`.
- [x] docs/postgres-backend.md — multi-tenant migration state, locks, sweep interplay, version capture.
- [x] CLAUDE.md — the two-hash rule (ShapeProjection vs RenderProjection), raw-write sanctioned-exception note.
- [ ] ~~docs/data-entry.md~~ (N/A: no UI in v1 beyond startup banner/log)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-DU4BUS (significant, addressed — A1), RR-5TYGFO
(significant, addressed — A2), RR-FVCHUA (significant, addressed — A3),
RR-FURO8P (significant, addressed — A4), RR-JPYXQ9 (significant, addressed —
A5), RR-P64QYC (minor, addressed — A6), RR-7IOBDB (minor, addressed — A7),
RR-3AGN9Y (minor, addressed — A8). All amendments recorded in TKT-0C57FS "Design
review amendments".
