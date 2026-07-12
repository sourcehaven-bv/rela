---
id: PLAN-W9WD26
type: planning-checklist
title: 'Planning: pgstore content versioning: time-machine history + diff with principal attribution'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented with test scenarios

**Scope:**

Motivated by RES-4ILUJZ scenarios: **S1 auditor** (faithful historical state +
attribution), **S2 editor recovering a bad change** (view/diff **+ restore**),
**S3 reviewer** (diff + who/when timeline).

**Capture model (revised twice; now HYBRID).** Two capture paths, chosen per op
because they have different information available:
- **Synchronous choke-point capture** for **rename** and **delete** (and the pre-delete final state) — these MUST be synchronous because only the entitymanager choke-point has the ground-truth the sweep can't reconstruct: rename knows `oldID→newID` (the sweep sees only a renamed entity indistinguishable from an update — RR-HKM0S6), and delete's row vanishes before any sweep runs. Both carry the real Principal from ctx.
- **Periodic reconciliation SWEEP** for **create/update** (the debounced common case): every X min, under a same-connection advisory lock, snapshot entities that have SETTLED (idle ≥ X min) OR exceeded a max-staleness ceiling, and whose content hash differs from their latest version. This is where debounce lives (a burst collapses to one version).

The sweep is NOT the sole writer — that framing was wrong (RR-HKM0S6). Sweep =
create/update debounce; synchronous = rename + delete.

IN scope (v1):
- Tables `entity_versions` (full snapshot) + `schema_versions` (content-addressed render-schema projection).
- Hybrid capture (above). Version rows retained after entity deletion (S1).
- Content-hash dedup; `schema_versions` projection-hash dedup.
- Optional `store.HistoryReader` (pgstore-only), type-asserted.
- CLI (postgres): `rela history <id>` + snapshot output for piping to external diff; `rela restore <id> <version>`.
- Data-entry: read API (list/get) + `<HistoryPanel>` + frontend diff; restore action.
- Render against the version's stored schema projection.
- Global permission `history:read` (deleted-entity history).
- Restore = field-validated PATCH (RR-VOYXRV). History reads through the serializer (RR-YDMJV7).

OUT of scope (v1) → follow-ups: relation history (**TKT-VFJKMB**); live-entity
schema_hash (**IDEA-ADI72Q**); intra-tx atomic capture (**TKT-N0OWKE**);
compliance purge (**TKT-BW6UUL**); Go diff lib; attachments; fsstore/memstore
versioning; MCP tool. Interval/ceiling tuning is config.

**Acceptance Criteria:**

1. **(S1) Faithful state + attribution + schema-faithful render.** History lists versions w/ principal/triggered-by/timestamp; a version renders as of its stored schema projection (change enum/`display_property` after → old version still old). → pgstore DB-gated + schema-drift render test.
2. **(S1) Deleted-entity history survives + gated + no oracle.** Delete → history returns rows incl. the synchronously-captured final pre-delete version; cascade-delete keeps version rows; deleted-history needs `history:read`; non-holder → same 404 as nonexistent (RR-KDXGYK). → pgstore + ACL tests.
3. **(S1/S3) Rename continuity.** Rename → `rela history <newid>` shows complete history across the rename incl. an `op=rename` event with old+new id, **captured synchronously at the rename choke-point** (RR-HKM0S6). → integration test incl. rename A→B→A and rename-then-reuse-of-id.
4. **(S2) Restore = field-validated** (RR-VOYXRV). Changed fields through `validateFieldWrite`; unwritable fields rejected/dropped; new attributed version; restoring a deleted entity re-creates it. → entitymanager + dataentry ACL tests.
5. **(field-ACL) History read redacts hidden fields** (RR-YDMJV7). Version response field set == live GET field set. → contract test mirroring `affordances_contract_test.go`.
6. **(S3) Diff.** CLI emits snapshots for external diff; frontend renders. → CLI golden + frontend test.
7. **Dedup / debounce.** Settled burst → one version; no-op write → no version; unchanged metamodel → one `schema_versions` row. → sweep counts test.
8. **(completeness) No starvation.** A continuously-edited entity (edited faster than the idle window) is STILL versioned within the max-staleness ceiling (RR-J188VJ). → sweep test with a never-settling edit loop.
9. **Backend gating.** Non-postgres: `rela history/restore` error clearly; postgres store lacking capability → clean "not supported". → build-tagged tests.
10. **(concurrency) Single-writer sweep — same-connection lock.** Two processes concurrently → no dup/lost rows; test asserts the whole tick (lock+select+insert+unlock) runs on ONE connection (RR-FB6QU8). → multi-connection integration test.
11. **(scale) Batch cap.** A bulk-import/post-migration burst does not produce an unbounded tick; the sweep drains in capped batches across ticks (RR-HLDQ6H). → test with N≫batch settled entities; EXPLAIN-assert the per-entity-probe plan.

## Research

- [x] `/research` → **RES-4ILUJZ** (done).
- [x] Five Explore surveys + THREE adversarial design reviews (concurrency; security; sweep-redesign).
- [x] Codebase reuse patterns confirmed.

**Prior art (reuse):** `internal/canonical` (hashes);
`openapi.computeMetamodelHash` (projection ~90%); pgstore listener goroutine
(lifecycle model, `listener.go`/`open.go`/`pgstore.go`); `pg_try_advisory_lock`
(single-runner — **same connection**, new key); `entities.updated_at` (settle
signal, `entity.go:221/263/393`, add index); `store.Formatter` (capability
type-assert); `rela db` (build-tag split); serializer
`forWire`/`stripHiddenProperties` (redaction, RR-YDMJV7); `validateFieldWrite`
(restore, RR-VOYXRV); indistinguishable-404 (RR-KDXGYK); `DocumentsPanel.vue`
(panel model); `graphquery_explain_test.go` (EXPLAIN-assert pattern, RR-HLDQ6H).

## Approach

- [x] Chosen and documented
- [x] Builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Design settled with user (2026-07-07) + two design-review rounds.**

*Storage (`0004_versions.sql`):*
- `schema_versions(hash TEXT COLLATE "C" PK, projection JSONB NOT NULL, captured_at TIMESTAMPTZ DEFAULT now())`; dedup `ON CONFLICT (hash) DO NOTHING`; never pruned.
- `entity_versions(entity_id TEXT COLLATE "C", vseq BIGINT DEFAULT nextval('version_seq'), op TEXT, prev_id TEXT NULL, type TEXT, content TEXT, properties JSONB, content_hash TEXT, schema_hash TEXT REFERENCES schema_versions(hash), principal_user TEXT, principal_tool TEXT, triggered_by TEXT, created_at TIMESTAMPTZ DEFAULT now(), PRIMARY KEY (entity_id, vseq))`. Ordering by dedicated **`version_seq`** sequence (NOT rela_seq — RR-E5AH72). Display "version N" = read-time row_number. **`op=rename` rows carry `prev_id`** (the old id) so cross-rename lineage is walkable at read time — this is captured SYNCHRONOUSLY (below), not derived by the sweep (RR-HKM0S6). Indexes: PK `(entity_id, vseq)`, `(entity_id, vseq DESC)` latest-probe, `entities(updated_at)` for the sweep filter.

*Capture — SYNCHRONOUS (rename + delete), at the entitymanager choke-point:*
- **Rename:** at `Manager.RenameEntity` (`manager.go:675`, alongside `recordRenameAudit`), insert an `op=rename` version row with `prev_id=oldID`, new id, real Principal from ctx (RR-HKM0S6).
- **Delete:** capture the final pre-delete version **BEFORE** `m.deps.Store.DeleteEntity` (i.e. before `manager.go:598`, using the in-memory `current` loaded at `:563`), NOT after at `:618` — order-before closes the permanent-loss window (RR-Q79T54). Real Principal from ctx. (Full intra-tx atomicity is TKT-N0OWKE.)

*Capture — SWEEP (create/update debounce), postgres build only:*
- Goroutine modeled on the listener (start `open.go`, stop `pgstore.go` Close; own lifetime ctx). Every X min:
  - **Acquire `pg_try_advisory_lock(<new key>)` and run the ENTIRE tick on that ONE connection** (via `pool.Acquire()` → use the `*pgxpool.Conn` for lock+select+insert+unlock). Never mix the lock connection with pool queries (RR-FB6QU8). If not acquired, another process sweeps — skip.
  - Select, **capped `ORDER BY updated_at LIMIT $batch`** (drain across ticks — RR-HLDQ6H): entities where (`updated_at < now()-$idle` **OR** latest-version `created_at < now()-$maxStaleness`) AND (no version OR latest `content_hash` ≠ current). The `$maxStaleness` ceiling defeats continuous-edit starvation (RR-J188VJ). Query shape = `DISTINCT ON`/`LATERAL … LIMIT 1` to force per-entity index probes; EXPLAIN-asserted.
  - For each: ensure `schema_versions` row (projection hash, cached per metamodel reload), insert `entity_versions` (op=create if first else update). Attribution = system principal `{tool: "version-sweep"}`; the *editing* principal is the audit log's job (do not duplicate — R2). Release lock.

*Read path:* `store.HistoryReader { ListVersions(ctx,id);
GetVersion(ctx,id,vseq) }`, pgstore-only, type-asserted. Render via `schema_hash
→ projection → render-only-metamodel view`. Lineage-across-rename walked via
`op=rename`/`prev_id`.

*Restore (RR-VOYXRV):* load snapshot → diff vs live → data-entry path runs
changed fields through `validateFieldWrite` (reject/drop unwritable) →
`entitymanager.UpdateEntity`/`CreateEntity`. Authorized+validated+audited. CLI
restore = operator-only.

*History read ACL (RR-YDMJV7, RR-KDXGYK):* live-entity history reuses
`gateReadOrNotFound`/`getVisible` (indistinguishable-404); deleted needs
`history:read` (non-holder → same 404). Each snapshot → `*entity.Entity` →
`entitySerializer.forWire` so `stripHiddenProperties` runs. `visible: when:`
evaluated against LIVE state; deleted → deny conditional.

*Permission `history:read`:* global-only; documented super-permission
(RR-D8NWM4).

*CLI/data-entry/frontend:* three-file build split (`history.go`, `restore.go` +
twins + `kong.go` + allowlist); dataentry list/get handlers + restore `_action`;
`HistoryPanel.vue` + JS diff.

**Files:** `pgstore/migrations/0004_versions.sql` (+`version_seq`,
`entities(updated_at)` idx); `pgstore/version.go` (HistoryReader + sync insert)
+ `sweep.go` (goroutine, same-conn lock, batch cap) + start/stop; `store.go`
(+HistoryReader+DTOs); `entitymanager/` (sync rename-capture at `manager.go:675`
+ delete-capture BEFORE `:598`); `metamodel/` (RenderProjection+hash+render-only
view); `acl/` (`history:read` + read gate); `appbuild_postgres.go` (start/stop
sweep; assert HistoryReader); `cli/{history,restore}.go`+twins; `dataentry/`
(handlers via forWire, restore via validateFieldWrite); `frontend/`
(HistoryPanel + diff + client + mount); docs.

## Security Considerations

- [x] Inputs identified; allowlist validation; sensitive ops identified; errors scrubbed
- **History read = content read** (RR-YDMJV7): snapshots through `forWire`; live-entity history reuses the read gate; contract test.
- **Deleted-entity history** (RR-KDXGYK, RR-D8NWM4): `history:read` (global); non-holder → indistinguishable 404; errors via `writeGateError`.
- **Restore = field-validated write** (RR-VOYXRV); fails-current-validation → rejected.
- **Point-in-time leak** (RR-A3RNT0): documented all-or-nothing-from-creation; compliance purge = TKT-BW6UUL.
- **Attribution** (RR-LE5DA2): principal from ctx only (sync capture) or system-principal (sweep); same trust model as audit; JWT resolver recommended.
- **Input validation:** id via `storeutil.ValidateID`; version ordinal range-checked.

## Test Plan

- [x] Per-AC scenarios; edge cases; negative tests; integration approach

**Scenarios:** AC1 multi-principal + schema-drift render; AC2 delete keeps
history + gate + 404-oracle; AC3 rename continuity (sync capture) incl. A→B→A +
rename-then-reuse; AC4 restore field-validation; AC5 field-visibility contract;
AC6 CLI golden + frontend; AC7 settled-burst dedup; **AC8 never-settling edit
loop still versioned within ceiling**; AC9 build-tag gating; **AC10
two-connection concurrent sweep, assert single-connection lock**; **AC11 N≫batch
drains across ticks + EXPLAIN-assert**. DB-gated `storetest`.

**Edge Cases:** empty content/props; >64KB line (BUG-LSBFD1); sweep idempotency
(2nd run inserts nothing); crash mid-sweep → lock auto-released, next tick
resumes, no corruption; rename A→B then create A → fresh lineage (create never
inherits); entity edited then deleted before sweep → sync delete-capture records
it; continuous edit → ceiling forces a version; `visible: when:` on deleted
entity → deny conditional; restore fails current validation → rejected.

**Negative:** unknown id → clean not-found; version out of range → clear error;
ACL-hidden live history → 404; deleted-history without `history:read` → **404
(not 403)**; restore unwritable field → rejected; field-`visible:` principal →
hidden fields absent.

## Risk Assessment

- [x] Technical + mitigations; security; effort

- **R1 (resolved)** per-write concurrency race → hybrid capture (sync rename/delete + advisory-locked single-connection sweep) + `version_seq`.
- **R2** swept-version attribution is system-principal; editing principal via audit log. Documented.
- **R3** sweep lag/crash = accepted debounce tolerance; **but** delete + rename are synchronous (order-before delete, RR-Q79T54) so they don't rely on the sweep.
- **R4** `entities(updated_at)` index add = non-CONCURRENT lock on a possibly-large table (`0003_sync.sql:38-46`); document the one-time stall.
- **R5** render-only-metamodel completeness → derive from actual render sites; test render with only the projection.
- **R6** hash instability → reuse `canonical`; golden-hash test.
- **R7** read-authz plumbing net-new (ACL write-only interface) → keep minimal.
- **R8** storage growth → debounce + dedup + retention cap (≥12mo); log prunes.
- **R9 (new)** sweep same-connection-lock discipline (RR-FB6QU8) — an implementation invariant; test asserts it.
- **R10 (new)** uncapped tick burst (RR-HLDQ6H) → batch cap + drain; EXPLAIN bench.

**Effort:** L (upper end). Sweep goroutine (same-conn lock, batch, ceiling) +
tables + reader (M); sync rename/delete capture (S); projection+hash (S); acl
gate (S); restore field-validation (S); CLI (S); data-entry
API+redaction+Vue+diff (M); tests/docs (M).

## Documentation Planning

- [x] User-facing docs identified; docs-checklist on entering implementation
- [x] `docs/postgres-backend.md` (tables, hybrid capture, sweep+ceiling+batch, retention, `updated_at` index)
- [x] `docs/cli-reference.md` (`rela history` +pipe / `rela restore` operator-only)
- [x] `docs/data-entry.md` (panel, diff, restore)
- [x] `docs/acl-security.md` (`history:read` super-permission + not-point-in-time caveat)
- [x] postgres `CLAUDE.md` (tables, hybrid capture model, `version_seq`, same-connection-lock invariant, attribution)

## Design Review

- [x] Ran `/design-review` TWICE (round 1: concurrency + security on the per-write design; round 2: the sweep redesign).
- [x] All critical/significant findings addressed in the plan.

**Round 1 findings (per-write design):** RR-7L5XBJ (crit, race) → redesign;
RR-YDMJV7 (crit, field redaction) → forWire; RR-VOYXRV (crit, restore field-ACL)
→ validated PATCH; RR-E5AH72 (sig, rela_seq) → version_seq; RR-OKNRDR (sig,
mapping) → **superseded by round 2** (derived lineage was wrong; now sync rename
capture); RR-A3RNT0 (sig, point-in-time) → docs + TKT-BW6UUL; RR-KDXGYK (sig,
oracle) → 404; RR-D8NWM4 (min) → docs; RR-LE5DA2 (min) → docs.

**Round 2 findings (sweep redesign):**
- **RR-HKM0S6 (critical)** sweep can't detect renames → **fixed**: rename captured synchronously at the choke-point with `prev_id`; sweep does create/update only.
- **RR-FB6QU8 (significant)** advisory lock session-scoped → **fixed**: whole tick on one acquired connection; invariant + test (AC-10).
- **RR-Q79T54 (significant)** delete-capture after committed delete → **fixed**: capture BEFORE `Store.DeleteEntity`.
- **RR-J188VJ (significant)** continuous-edit starvation → **fixed**: max-staleness ceiling + AC-8.
- **RR-HLDQ6H (significant)** uncapped tick → **fixed**: batch cap + drain + EXPLAIN-assert (AC-11).
- **RR-3L2O7Y (minor)** relation-churn decoupling + automation attribution → **documented**.
